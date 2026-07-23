package wit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/p2p"

	ethsdk "github.com/nethoxa-labs/raidan-sdk/eth"
	sdkrlpx "github.com/nethoxa-labs/raidan-sdk/rlpx"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// ErrUnsupported reports that the peer did not negotiate WIT/1.
var ErrUnsupported = errors.New("peer does not advertise wit/1")

// Conn is a negotiated WIT/1 connection.
type Conn struct {
	ctx       context.Context
	preStatus *ethsdk.PreStatusConn
	offset    uint64
}

// Dial negotiates ETH/68 or ETH/69 and WIT/1, then completes the ETH Status
// exchange. The returned connection is ready for WIT messages.
func Dial(ctx context.Context, target, rpc string) (*Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	caps := []p2p.Cap{
		{Name: "eth", Version: 69},
		{Name: "eth", Version: 68},
		{Name: "wit", Version: 1},
	}
	preStatus, err := ethsdk.DialPreStatus(ctx, target, rpc, ethsdk.Config{Capabilities: caps})
	if err != nil {
		return nil, err
	}
	if !hasCapability(preStatus.PeerCapabilities(), "wit", 1) {
		preStatus.Close()
		return nil, ErrUnsupported
	}

	if err := preStatus.ExchangeStatus(10 * time.Second); err != nil {
		preStatus.Close()
		return nil, err
	}

	ethLength, err := ethsdk.ProtocolLength(preStatus.ETHVersion())
	if err != nil {
		preStatus.Close()
		return nil, err
	}
	return &Conn{
		ctx:       ctx,
		preStatus: preStatus,
		offset:    preStatus.ETHOffset() + ethLength,
	}, nil
}

// Close tears down the underlying connection.
func (c *Conn) Close() { c.preStatus.Close() }

// SendRaw writes an already RLP-encoded WIT message. The observer callback is
// invoked immediately before the bytes are written to the wire.
func (c *Conn) SendRaw(code uint64, payload []byte) error {
	session.ObserveWrite(c.ctx, session.Write{Protocol: "wit", Code: code, Payload: payload})
	_, err := c.preStatus.RLPx().Write(c.offset+code, payload)
	return err
}

// ReadRaw reads the next WIT/1 message and returns its protocol-relative code.
// Devp2p keepalive messages are handled internally and unrelated negotiated
// protocol messages are skipped until the deadline expires.
func (c *Conn) ReadRaw(timeout time.Duration) (uint64, []byte, error) {
	if c == nil || c.preStatus == nil {
		return 0, nil, errors.New("nil WIT connection")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	rlpxConn := c.preStatus.RLPx()
	if err := rlpxConn.SetReadDeadline(time.Now().Add(session.Timeout(c.ctx, timeout))); err != nil {
		return 0, nil, fmt.Errorf("set WIT read deadline: %w", err)
	}
	defer func() { _ = rlpxConn.SetReadDeadline(time.Time{}) }()
	for {
		code, payload, _, err := rlpxConn.Read()
		if err != nil {
			return 0, nil, err
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(payload)
			return 0, nil, errors.New("peer disconnected: " + reason)
		case sdkrlpx.PingMsg:
			if _, err := rlpxConn.Write(sdkrlpx.PongMsg, nil); err != nil {
				return 0, nil, fmt.Errorf("write WIT session pong: %w", err)
			}
			continue
		}
		if code < c.offset || code >= c.offset+4 {
			continue
		}
		return code - c.offset, append([]byte(nil), payload...), nil
	}
}

// WaitDisconnect waits for the peer to close or send a devp2p Disconnect.
// A local timeout returns closed=false.
func (c *Conn) WaitDisconnect(timeout time.Duration) (closed bool, reason string) {
	return c.preStatus.WaitDisconnect(timeout)
}

func hasCapability(caps []p2p.Cap, name string, version uint) bool {
	for _, cap := range caps {
		if cap.Name == name && cap.Version == version {
			return true
		}
	}
	return false
}
