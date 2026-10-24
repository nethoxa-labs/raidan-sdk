package mplex

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multistream"

	"github.com/nethoxa-labs/raidan-sdk/reqresp"
	"github.com/nethoxa-labs/raidan-sdk/session"
	"github.com/nethoxa-labs/raidan-sdk/target"
)

// ProtocolID is the only stream multiplexer this package negotiates.
const ProtocolID = "/mplex/6.7.0"

const (
	// MaxSend caps the bytes the connection writes after the multiplexer is negotiated.
	MaxSend = 64 << 20
	// MaxFrame is the largest frame payload (the mplex limit, 1 MiB).
	MaxFrame = 1 << 20

	dialTimeout = 10 * time.Second
)

// Frame flags. The mplex header is (stream ID << 3) | flag.
const (
	FlagOpen          = 0
	FlagDataReceiver  = 1
	FlagDataInitiator = 2
	FlagCloseReceiver = 3
	FlagCloseInit     = 4
	FlagResetReceiver = 5
	FlagResetInit     = 6
)

// Frame is one frame read from the peer.
type Frame struct {
	Stream  uint64
	Flag    uint64
	Payload []byte
}

// Conn is the negotiated mplex connection to the peer.
type Conn struct {
	ctx      context.Context
	conn     net.Conn
	sent     uint64
	received uint64
	limit    uint64
	buf      []byte
}

// Dial connects to t.ConsensusP2P with the current case identity. It returns
// once /mplex/6.7.0 is negotiated. Any other outcome is an error. The peer can
// select another multiplexer or refuse mplex. maxSend bounds the outbound bytes
// after negotiation (0 means MaxSend).
func Dial(ctx context.Context, t target.Target, maxSend uint64) (*Conn, error) {
	participant, err := session.ParticipantIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Raidan participant key: %w", err)
	}
	key, err := reqresp.CLKey(participant)
	if err != nil {
		return nil, fmt.Errorf("load Raidan participant key: %w", err)
	}
	return DialWithIdentity(ctx, t, key, maxSend)
}

// DialWithIdentity is Dial with an explicit local identity. Multi-identity
// cases must opt in explicitly through this path.
func DialWithIdentity(ctx context.Context, t target.Target, key libp2pcrypto.PrivKey, maxSend uint64) (*Conn, error) {
	if key == nil {
		return nil, errors.New("mplex local identity is nil")
	}
	if maxSend == 0 || maxSend > MaxSend {
		maxSend = MaxSend
	}
	id, err := reqresp.PeerID(ctx, t.ConsensusURL, t.ConsensusP2P)
	if err != nil {
		return nil, err
	}
	addr, err := reqresp.Multiaddr(t.ConsensusP2P, id)
	if err != nil {
		return nil, err
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("parse peer multiaddr: %w", err)
	}
	var dest net.Addr
	for _, a := range info.Addrs {
		if _, err := a.ValueForProtocol(ma.P_TCP); err != nil {
			continue
		}
		if dest, err = manet.ToNetAddr(a); err != nil {
			return nil, err
		}
		break
	}
	if dest == nil {
		return nil, errors.New("the peer has no TCP address")
	}
	dialCtx, cancel := context.WithTimeout(ctx, session.Remaining(ctx, dialTimeout))
	defer cancel()
	raw, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", dest.String())
	if err != nil {
		return nil, fmt.Errorf("dial peer: %w", err)
	}
	fail := func(format string, args ...any) (*Conn, error) {
		_ = raw.Close()
		return nil, fmt.Errorf(format, args...)
	}
	if deadline, ok := dialCtx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if err := multistream.SelectProtoOrFail(noise.ID, raw); err != nil {
		return fail("multistream-select %s: %w", noise.ID, err)
	}
	transport, err := noise.New(noise.ID, key, nil)
	if err != nil {
		return fail("noise: %w", err)
	}
	secured, err := transport.SecureOutbound(dialCtx, raw, id)
	if err != nil {
		return fail("noise handshake: %w", err)
	}
	if err := multistream.SelectProtoOrFail(ProtocolID, secured); err != nil {
		_ = secured.Close()
		return nil, fmt.Errorf("the peer did not accept %s: %w", ProtocolID, err)
	}
	_ = secured.SetDeadline(time.Time{})
	return &Conn{ctx: ctx, conn: secured, limit: maxSend}, nil
}

// Open sends an Open frame for stream id with the given name.
func (c *Conn) Open(id uint64, name string) error { return c.send(id, FlagOpen, []byte(name)) }

// Data sends a Data frame (initiator side) on stream id.
func (c *Conn) Data(id uint64, payload []byte) error { return c.send(id, FlagDataInitiator, payload) }

// Close sends a Close frame (initiator side) for stream id.
func (c *Conn) Close(id uint64) error { return c.send(id, FlagCloseInit, nil) }

// Reset sends a Reset frame (initiator side) for stream id.
func (c *Conn) Reset(id uint64) error { return c.send(id, FlagResetInit, nil) }

// Frame sends one frame with a caller-chosen flag (0 to 6). Use it for a flag
// that the helpers above do not cover.
func (c *Conn) Frame(id, flag uint64, payload []byte) error {
	if flag > FlagResetInit {
		return fmt.Errorf("mplex flag %d is not defined", flag)
	}
	return c.send(id, flag, payload)
}

func (c *Conn) send(id, flag uint64, payload []byte) error {
	if len(payload) > MaxFrame {
		return fmt.Errorf("mplex payload of %d bytes exceeds %d", len(payload), MaxFrame)
	}
	if id > 1<<60 {
		return errors.New("mplex stream id is out of range")
	}
	frame := binary.AppendUvarint(nil, id<<3|flag)
	frame = binary.AppendUvarint(frame, uint64(len(payload)))
	frame = append(frame, payload...)
	if c.sent+uint64(len(frame)) > c.limit {
		return fmt.Errorf("outbound cap of %d bytes reached", c.limit)
	}
	session.ObserveWrite(c.ctx, session.Write{Protocol: "mplex", Code: flag, ProtocolID: ProtocolID, Payload: frame, Raw: true})
	_ = c.conn.SetWriteDeadline(time.Now().Add(session.Remaining(c.ctx, 30*time.Second)))
	n, err := c.conn.Write(frame)
	c.sent += uint64(n)
	return err
}

// Drain reads the peer's frames for up to d and returns them. If the peer closes
// the connection, the read ends with its error after the frames already read.
// Between Drain calls nothing is read, which is the backpressure.
func (c *Conn) Drain(d time.Duration) ([]Frame, error) {
	d = min(d, session.Remaining(c.ctx, d))
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	var frames []Frame
	chunk := make([]byte, 64<<10)
	for {
		n, err := c.conn.Read(chunk)
		c.received += uint64(n)
		c.buf = append(c.buf, chunk[:n]...)
		for {
			f, used := parse(c.buf)
			if used == 0 {
				break
			}
			frames = append(frames, f)
			c.buf = c.buf[used:]
		}
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				return frames, nil
			}
			return frames, err
		}
	}
}

func parse(b []byte) (Frame, int) {
	header, n1 := binary.Uvarint(b)
	if n1 <= 0 {
		return Frame{}, 0
	}
	size, n2 := binary.Uvarint(b[n1:])
	if n2 <= 0 || size > MaxFrame || uint64(len(b)-n1-n2) < size {
		return Frame{}, 0
	}
	start := n1 + n2
	return Frame{Stream: header >> 3, Flag: header & 7, Payload: append([]byte(nil), b[start:start+int(size)]...)}, start + int(size)
}

// Sent and Received are the exact bytes written to and read from the multiplexed (encrypted) connection since
// negotiation, counted as the plaintext mplex bytes.
func (c *Conn) Sent() uint64     { return c.sent }
func (c *Conn) Received() uint64 { return c.received }

// Negotiated is the multiplexer the connection runs; it is always ProtocolID, since Dial fails otherwise.
func (c *Conn) Negotiated() string { return ProtocolID }

// Shut closes the connection.
func (c *Conn) Shut() error { return c.conn.Close() }
