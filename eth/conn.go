package eth

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"

	sdkrlpx "github.com/nethoxa-labs/raidan-sdk/rlpx"
	ethrpc "github.com/nethoxa-labs/raidan-sdk/rpc"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

type connMsg struct {
	code uint64
	data []byte
}

var errReceiveQueueFull = errors.New("eth receive queue full")

// WireReadWriter is the framed RLPx subset used by negotiated ETH sessions.
type WireReadWriter interface {
	Read() (uint64, []byte, int, error)
	Write(uint64, []byte) (uint32, error)
}

// ReadDeadlineSetter is the socket subset needed by bounded wire reads.
type ReadDeadlineSetter interface {
	SetReadDeadline(time.Time) error
}

// Conn is a lightweight devp2p+ETH connection backed by a raw rlpx.Conn.
// Multiple Conns can run in parallel — each is just a TCP socket.
type Conn struct {
	ctx        context.Context
	rlpxConn   *sdkrlpx.Conn
	fd         net.Conn
	ethOffset  uint64
	snapOffset uint64
	ethVer     uint
	params     ethrpc.ChainParams
	dead       chan struct{}
	msgs       chan connMsg
	writeMu    sync.Mutex
	discMu     sync.RWMutex
	discReason string
	readErrMu  sync.RWMutex
	readErr    error
}

// PreStatusConn is an RLPx connection whose Hello exchange is complete but
// whose ETH Status exchange remains under caller control for custom handshakes.
type PreStatusConn struct {
	ctx              context.Context
	rlpxConn         *sdkrlpx.Conn
	fd               net.Conn
	params           ethrpc.ChainParams
	ethVersion       uint
	key              *ecdsa.PrivateKey
	ethOffset        uint64
	snapOffset       uint64
	peerCapabilities []p2p.Cap
}

// DialPreStatus performs TCP, RLPx, and Hello negotiation and stops before
// ETH Status. Capabilities defaults to eth/69 and eth/68; supplying
// Config.Capabilities uses that exact ordered list.
func DialPreStatus(ctx context.Context, target, rpcURL string, config Config) (*PreStatusConn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("eth config: %w", err)
	}
	node, err := enode.ParseV4(target)
	if err != nil {
		return nil, fmt.Errorf("parse enode: %w", err)
	}

	var params ethrpc.ChainParams
	if rpcURL != "" {
		chainParams, err := ethrpc.FetchChainParams(ctx, rpcURL)
		if err != nil {
			return nil, fmt.Errorf("fetch chain params: %w", err)
		}
		params = chainParams
	}

	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	rlpxConn, fd, err := sdkrlpx.DialWithKey(ctx, target, key)
	if err != nil {
		return nil, fmt.Errorf("rlpx handshake: %w", err)
	}
	closeOnError := func(err error) (*PreStatusConn, error) {
		_ = fd.Close()
		return nil, err
	}
	if err := fd.SetDeadline(time.Now().Add(session.Timeout(ctx, 10*time.Second))); err != nil {
		return closeOnError(fmt.Errorf("set hello deadline: %w", err))
	}

	caps := config.capabilities()
	if config.MaxVersion == 0 && config.SnapVersion == 0 && len(config.Capabilities) == 0 {
		// Keep the pre-Status default independent of Dial's eth/70 default.
		caps = ethCapabilities(69)
	}
	hello := protoHandshake{
		Version: 5,
		Name:    "raidan-sdk",
		Caps:    caps,
		ID:      crypto.FromECDSAPub(&key.PublicKey)[1:],
	}
	helloData, err := rlp.EncodeToBytes(&hello)
	if err != nil {
		return closeOnError(fmt.Errorf("encode hello: %w", err))
	}
	if _, err := rlpxConn.Write(sdkrlpx.HelloMsg, helloData); err != nil {
		return closeOnError(fmt.Errorf("write hello: %w", err))
	}

	code, data, _, err := rlpxConn.Read()
	if err != nil {
		return closeOnError(fmt.Errorf("read hello: %w", err))
	}
	if code == sdkrlpx.DiscMsg {
		_, reason := sdkrlpx.DecodeDisconnectReason(data)
		return closeOnError(errors.New("disconnected during hello: " + reason))
	}
	if code != sdkrlpx.HelloMsg {
		return closeOnError(fmt.Errorf("expected hello, got 0x%02x", code))
	}

	var peerHello protoHandshake
	if err := rlp.DecodeBytes(data, &peerHello); err != nil {
		return closeOnError(fmt.Errorf("decode hello: %w", err))
	}
	if err := validateHelloIdentity(node, peerHello.ID); err != nil {
		return closeOnError(err)
	}
	if peerHello.Version >= 5 {
		rlpxConn.SetSnappy(true)
	}

	ethVersion := highestCommonEthVersion(caps, peerHello.Caps)
	if ethVersion == 0 {
		return closeOnError(errors.New("no common eth version"))
	}
	ethOffset, _, ok, err := capabilityOffset(caps, peerHello.Caps, "eth")
	if err != nil {
		return closeOnError(fmt.Errorf("compute ETH capability offset: %w", err))
	}
	if !ok {
		return closeOnError(errors.New("negotiated ETH capability has no canonical offset"))
	}
	snapOffset, _, _, err := capabilityOffset(caps, peerHello.Caps, "snap")
	if err != nil {
		return closeOnError(fmt.Errorf("compute SNAP capability offset: %w", err))
	}
	if err := fd.SetDeadline(time.Time{}); err != nil {
		return closeOnError(fmt.Errorf("clear handshake deadline: %w", err))
	}

	return &PreStatusConn{
		ctx:              ctx,
		rlpxConn:         rlpxConn,
		fd:               fd,
		params:           params,
		ethVersion:       ethVersion,
		key:              key,
		ethOffset:        ethOffset,
		snapOffset:       snapOffset,
		peerCapabilities: slices.Clone(peerHello.Caps),
	}, nil
}

// RLPx returns the negotiated connection for caller-controlled framed I/O.
func (c *PreStatusConn) RLPx() *sdkrlpx.Conn { return c.rlpxConn }

// NetConn returns the underlying TCP connection for deadline control.
func (c *PreStatusConn) NetConn() net.Conn { return c.fd }

// Params returns a copy of the chain parameters fetched during dialing.
func (c *PreStatusConn) Params() ethrpc.ChainParams { return c.params.Clone() }

// ETHVersion returns the negotiated ETH protocol version.
func (c *PreStatusConn) ETHVersion() uint { return c.ethVersion }

// PrivateKey returns the local identity key used for RLPx and Hello.
func (c *PreStatusConn) PrivateKey() *ecdsa.PrivateKey { return c.key }

// PeerCapabilities returns a copy of the peer's advertised capabilities.
func (c *PreStatusConn) PeerCapabilities() []p2p.Cap {
	return slices.Clone(c.peerCapabilities)
}

// ETHOffset returns the absolute wire offset for ETH messages.
func (c *PreStatusConn) ETHOffset() uint64 { return c.ethOffset }

// SNAPOffset returns the absolute SNAP wire offset, or zero if unavailable.
func (c *PreStatusConn) SNAPOffset() uint64 { return c.snapOffset }

// SendRaw writes an already encoded payload at an absolute wire code.
func (c *PreStatusConn) SendRaw(wireCode uint64, payload []byte) error {
	session.ObserveWrite(c.ctx, session.Write{Protocol: "rlpx", Code: wireCode, Payload: payload})
	_, err := c.rlpxConn.Write(wireCode, payload)
	return err
}

// SendETH encodes and writes an ETH message.
func (c *PreStatusConn) SendETH(code uint64, message any) error {
	payload, err := rlp.EncodeToBytes(message)
	if err != nil {
		return err
	}
	return c.SendETHRaw(code, payload)
}

// SendETHRaw writes an already encoded ETH payload.
func (c *PreStatusConn) SendETHRaw(code uint64, payload []byte) error {
	session.ObserveWrite(c.ctx, session.Write{Protocol: "eth", Code: code, Payload: payload})
	_, err := c.rlpxConn.Write(c.ethOffset+code, payload)
	return err
}

// WaitDisconnect reports whether the peer closed and includes its decoded
// devp2p reason when one was sent. A deadline expiry is not a close.
func (c *PreStatusConn) WaitDisconnect(timeout time.Duration) (bool, string) {
	return waitStatusOutcome(c.ctx, c.rlpxConn, c.fd, timeout)
}

// Close tears down the connection.
func (c *PreStatusConn) Close() { _ = c.fd.Close() }

// Dial performs RLPx, Hello, and Status and returns a ready ETH connection.
// MaxVersion defaults to 70, matching the broadly deployed protocol baseline.
func Dial(ctx context.Context, target, rpc string, config Config) (*Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("eth config: %w", err)
	}
	node, err := enode.ParseV4(target)
	if err != nil {
		return nil, fmt.Errorf("parse enode: %w", err)
	}

	cp, err := ethrpc.FetchChainParams(ctx, rpc)
	if err != nil {
		return nil, fmt.Errorf("fetch chain params: %w", err)
	}
	// Fork-ID recovery below is specific to this dial. Keep it isolated from
	// cached chain parameters and concurrent connections.
	dialParams := cp

	caps := config.capabilities()
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	conn, peerFID, dialErr := rawDial(ctx, node, &dialParams, key, caps)
	if dialErr == nil {
		go conn.reader()
		return conn, nil
	}

	// A peer Status received before a fork-ID rejection lets us retry once with
	// the peer's view. Other failures stay under the caller's retry policy.
	if peerFID == (forkid.ID{}) {
		return nil, dialErr
	}
	dialParams.ForkID = peerFID
	key, err = crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("create retry identity: %w", err)
	}
	conn, _, dialErr = rawDial(ctx, node, &dialParams, key, caps)
	if dialErr != nil {
		return nil, dialErr
	}
	go conn.reader()
	return conn, nil
}

// Send writes an ETH-level message. The code is the ETH message code
// (e.g. 0x02 for Transactions, 0x0B for an unknown code). The wire
// offset is added automatically.
func (c *Conn) Send(code uint64, data any) error {
	payload, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}
	_, err = c.writeObserved("eth", code, c.ethOffset+code, payload)
	return err
}

// SendRaw writes a pre-encoded ETH-level message. The wire offset is
// added automatically. Use this to avoid re-encoding the same payload
// in tight loops.
func (c *Conn) SendRaw(code uint64, payload []byte) error {
	_, err := c.writeObserved("eth", code, c.ethOffset+code, payload)
	return err
}

// WriteRaw writes a pre-encoded message with an absolute wire code
// (no offset added). Use for sending on arbitrary protocol codes.
func (c *Conn) WriteRaw(wireCode uint64, payload []byte) error {
	_, err := c.writeObserved("rlpx", wireCode, wireCode, payload)
	return err
}

// RLPx exposes the underlying framed connection.
func (c *Conn) RLPx() *sdkrlpx.Conn { return c.rlpxConn }

// SendSnap writes a SNAP-level message. Returns an error if snap was
// not negotiated during the handshake.
func (c *Conn) SendSnap(code uint64, data any) error {
	if c.snapOffset == 0 {
		return errors.New("snap not negotiated")
	}
	payload, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}
	_, err = c.writeObserved("snap", code, c.snapOffset+code, payload)
	return err
}

// SendSnapRaw writes a pre-encoded SNAP-level message. Returns an error
// if snap was not negotiated during the handshake.
func (c *Conn) SendSnapRaw(code uint64, payload []byte) error {
	if c.snapOffset == 0 {
		return errors.New("snap not negotiated")
	}
	_, err := c.writeObserved("snap", code, c.snapOffset+code, payload)
	return err
}

func (c *Conn) writeObserved(protocol string, code, wireCode uint64, payload []byte) (uint32, error) {
	session.ObserveWrite(c.ctx, session.Write{Protocol: protocol, Code: code, Payload: payload})
	return c.writeWire(c.rlpxConn, wireCode, payload)
}

func (c *Conn) writeWire(wire WireReadWriter, code uint64, payload []byte) (uint32, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return wire.Write(code, payload)
}

// ReadMsg returns the next message from the peer (excluding ping/pong/disc
// which are handled internally). Returns the raw wire code, data, and error.
func (c *Conn) ReadMsg(timeout time.Duration) (uint64, []byte, error) {
	timeout = session.Timeout(c.ctx, timeout)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case msg := <-c.msgs:
		return msg.code, msg.data, nil
	case <-c.dead:
		return 0, nil, c.readFailure()
	case <-c.ctx.Done():
		return 0, nil, c.ctx.Err()
	case <-timer.C:
		return 0, nil, errors.New("timeout")
	}
}

// WaitForMsg reads messages until one with the expected wire code arrives,
// discarding any others. Returns the message data.
func (c *Conn) WaitForMsg(timeout time.Duration, wantCode uint64) ([]byte, error) {
	timer := time.NewTimer(session.Timeout(c.ctx, timeout))
	defer timer.Stop()
	for {
		select {
		case msg := <-c.msgs:
			if msg.code == wantCode {
				return msg.data, nil
			}
		case <-c.dead:
			return nil, c.readFailure()
		case <-c.ctx.Done():
			// See ReadMsg: a cancelled run unblocks the wait immediately
			// rather than letting it run out the full window.
			return nil, errors.New("run cancelled")
		case <-timer.C:
			return nil, errors.New("timeout")
		}
	}
}

// WaitDisconnect blocks until the peer disconnects or the timeout expires.
// Returns true if the peer disconnected.
func (c *Conn) WaitDisconnect(timeout time.Duration) bool {
	timer := time.NewTimer(session.Timeout(c.ctx, timeout))
	defer timer.Stop()
	select {
	case <-c.dead:
		return true
	case <-c.ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

// ETHOffset returns the wire offset for ETH messages.
func (c *Conn) ETHOffset() uint64 { return c.ethOffset }

// SNAPOffset returns the wire offset for SNAP messages (0 if not negotiated).
func (c *Conn) SNAPOffset() uint64 { return c.snapOffset }

// ETHVersion returns the negotiated ETH protocol version.
func (c *Conn) ETHVersion() uint { return c.ethVer }

// Closed reports whether the peer connection has terminated without waiting.
func (c *Conn) Closed() bool {
	select {
	case <-c.dead:
		return true
	default:
		return false
	}
}

// Params returns the chain parameters used for the handshake.
func (c *Conn) Params() ethrpc.ChainParams { return c.params.Clone() }

// Close tears down the connection.
func (c *Conn) Close() {
	_ = c.fd.Close()
}

func (c *Conn) setDisconnectReason(reason string) {
	if reason == "" {
		return
	}
	c.discMu.Lock()
	if c.discReason == "" {
		c.discReason = reason
	}
	c.discMu.Unlock()
}

// DisconnectReason returns the decoded reason from a peer-sent devp2p
// Disconnect message, or "" when the socket closed without one.
func (c *Conn) DisconnectReason() string {
	c.discMu.RLock()
	defer c.discMu.RUnlock()
	return c.discReason
}

// DisconnectDetail returns a human-readable close detail suitable for
// errors and outcome strings.
func (c *Conn) DisconnectDetail() string {
	return sdkrlpx.DisconnectDetail(c.DisconnectReason())
}

func (c *Conn) setReadError(err error) {
	if err == nil {
		return
	}
	c.readErrMu.Lock()
	if c.readErr == nil {
		c.readErr = err
	}
	c.readErrMu.Unlock()
}

func (c *Conn) readFailure() error {
	c.readErrMu.RLock()
	err := c.readErr
	c.readErrMu.RUnlock()
	if err != nil {
		return err
	}
	return errors.New(c.DisconnectDetail())
}

func (c *Conn) queueMessage(msg connMsg) bool {
	select {
	case c.msgs <- msg:
		return true
	default:
		c.setReadError(errReceiveQueueFull)
		return false
	}
}

// reader runs in a background goroutine, responding to pings and detecting
// disconnects. Routes other messages to c.msgs for ReadMsg consumers.
func (c *Conn) reader() {
	c.readMessages(c.rlpxConn)
}

func (c *Conn) readMessages(wire WireReadWriter) {
	defer func() {
		select {
		case <-c.dead:
		default:
			close(c.dead)
		}
	}()
	for {
		code, data, _, err := wire.Read()
		if err != nil {
			c.setReadError(fmt.Errorf("read message: %w", err))
			return
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			c.setDisconnectReason(reason)
			return
		case sdkrlpx.PingMsg:
			if _, err := c.writeWire(wire, sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				c.setReadError(fmt.Errorf("write pong: %w", err))
				_ = c.fd.Close()
				return
			}
		case sdkrlpx.PongMsg:
			// Base-protocol keepalive acknowledgement; never expose it as ETH data.
		default:
			if !c.queueMessage(connMsg{code, data}) {
				// A bounded fail-fast is safer than either blocking the wire reader
				// or silently losing a response that a caller may be waiting for.
				_ = c.fd.Close()
				return
			}
		}
	}
}

// rawDial performs the full connection sequence: TCP → RLPx → Hello → ETH Status.
// Returns the live connection plus the peer's fork ID (even on error, if available).
//
// The TCP dial honors ctx so cancellation returns in-flight handshakes instead
// of leaving them pinned on the timeout.
func rawDial(ctx context.Context, node *enode.Node, cp *ethrpc.ChainParams, key *ecdsa.PrivateKey, caps []p2p.Cap) (*Conn, forkid.ID, error) {
	rc, fd, err := sdkrlpx.DialWithKey(ctx, node.String(), key)
	if err != nil {
		return nil, forkid.ID{}, fmt.Errorf("rlpx handshake: %w", err)
	}
	if err := fd.SetDeadline(time.Now().Add(session.Timeout(ctx, 10*time.Second))); err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("set hello deadline: %w", err)
	}

	// ── devp2p Hello ────────────────────────────────────────────────────

	hello := protoHandshake{
		Version: 5,
		Name:    "raidan-sdk",
		Caps:    caps,
		ID:      crypto.FromECDSAPub(&key.PublicKey)[1:],
	}
	helloData, err := rlp.EncodeToBytes(&hello)
	if err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("encode hello: %w", err)
	}
	if _, err := rc.Write(sdkrlpx.HelloMsg, helloData); err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("write hello: %w", err)
	}

	code, data, _, err := rc.Read()
	if err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("read hello: %w", err)
	}
	if code == sdkrlpx.DiscMsg {
		_, reason := sdkrlpx.DecodeDisconnectReason(data)
		_ = fd.Close()
		return nil, forkid.ID{}, errors.New("disconnected during hello: " + reason)
	}
	if code != sdkrlpx.HelloMsg {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("expected hello (0x00), got 0x%02x", code)
	}

	var peerHello protoHandshake
	if err := rlp.DecodeBytes(data, &peerHello); err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("decode hello: %w", err)
	}
	if err := validateHelloIdentity(node, peerHello.ID); err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, err
	}

	// Enable Snappy compression (devp2p v5+)
	if peerHello.Version >= 5 {
		rc.SetSnappy(true)
	}

	// Negotiate highest common eth version.
	ethVer := highestCommonEthVersion(caps, peerHello.Caps)
	if ethVer == 0 {
		_ = fd.Close()
		return nil, forkid.ID{}, errors.New("no common eth version")
	}
	if err := fd.SetDeadline(time.Time{}); err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("clear hello deadline: %w", err)
	}

	// Check snap support
	ethOffset, _, ok, err := capabilityOffset(caps, peerHello.Caps, "eth")
	if err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("compute ETH capability offset: %w", err)
	}
	if !ok {
		_ = fd.Close()
		return nil, forkid.ID{}, errors.New("negotiated ETH capability has no canonical offset")
	}
	snapOffset, _, _, err := capabilityOffset(caps, peerHello.Caps, "snap")
	if err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("compute SNAP capability offset: %w", err)
	}

	// ── ETH Status ──────────────────────────────────────────────────────

	statusData, err := encodeStatus(ethVer, cp)
	if err != nil {
		_ = fd.Close()
		return nil, forkid.ID{}, fmt.Errorf("encode status: %w", err)
	}

	// Send concurrently to avoid geth's 5s handshake timeout
	sendErr := make(chan error, 1)
	go func() {
		_, err := rc.Write(ethOffset, statusData)
		sendErr <- err
	}()

	// Read peer's Status
	var peerFID forkid.ID
	if err := fd.SetDeadline(time.Now().Add(session.Timeout(ctx, 10*time.Second))); err != nil {
		_ = fd.Close()
		return nil, peerFID, fmt.Errorf("set status deadline: %w", err)
	}
readStatus:
	for {
		code, data, _, err = rc.Read()
		if err != nil {
			_ = fd.Close()
			return nil, peerFID, fmt.Errorf("read status: %w", err)
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			_ = fd.Close()
			return nil, peerFID, errors.New("disconnected during status exchange: " + reason)
		case ethOffset: // ETH Status
			peerFID, err = decodeAndValidateStatus(ethVer, data, cp)
			if err != nil {
				_ = fd.Close()
				return nil, peerFID, fmt.Errorf("decode status: %w", err)
			}
			break readStatus
		case sdkrlpx.PingMsg:
			if _, err := rc.Write(sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				_ = fd.Close()
				return nil, peerFID, fmt.Errorf("write pong: %w", err)
			}
		case sdkrlpx.PongMsg:
			// Keepalive acknowledgements are handled by the session.
		}
	}
	if err := fd.SetDeadline(time.Time{}); err != nil {
		_ = fd.Close()
		return nil, peerFID, fmt.Errorf("clear status deadline: %w", err)
	}

	if err := <-sendErr; err != nil {
		_ = fd.Close()
		return nil, peerFID, fmt.Errorf("write status: %w", err)
	}

	return &Conn{
		ctx:        ctx,
		rlpxConn:   rc,
		fd:         fd,
		ethOffset:  ethOffset,
		snapOffset: snapOffset,
		ethVer:     ethVer,
		params:     *cp,
		dead:       make(chan struct{}),
		msgs:       make(chan connMsg, 256),
	}, peerFID, nil
}
