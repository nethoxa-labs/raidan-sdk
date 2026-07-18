package discovery

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
)

// Discv4PeerHarness keeps one local discovery identity across a bonded exchange.
type Discv4PeerHarness struct {
	Conn *Discv4Conn
	peer Discv4Endpoint
}

// NewDiscv4PeerHarness opens a connection for a multi-packet discv4 exchange.
func NewDiscv4PeerHarness(ctx context.Context, target string) (*Discv4PeerHarness, error) {
	u, err := DialDiscv4(ctx, target)
	if err != nil {
		return nil, err
	}
	return newDiscv4PeerHarness(target, u)
}

func newDiscv4PeerHarness(target string, u *Discv4Conn) (*Discv4PeerHarness, error) {
	peer, err := PeerEndpoint(target)
	if err != nil {
		_ = u.Close()
		return nil, err
	}
	return &Discv4PeerHarness{Conn: u, peer: peer}, nil
}

// Close releases the harness connection.
func (h *Discv4PeerHarness) Close() error {
	return h.Conn.Close()
}

// SendPacket signs and sends one packet and returns its hash.
func (h *Discv4PeerHarness) SendPacket(ptype byte, data any) ([]byte, error) {
	return h.Conn.sendDiscv4Packet(ptype, data)
}

// SendPing sends a canonical Ping and returns its packet hash.
func (h *Discv4PeerHarness) SendPing() ([]byte, error) {
	ping := Discv4Ping{
		Version:    4,
		From:       EndpointOf(h.Conn.LocalAddr()),
		To:         h.peer,
		Expiration: uint64(time.Now().Add(30 * time.Second).Unix()),
	}
	return h.SendPacket(Discv4PingType, &ping)
}

// ReplyToPing decodes pkt and answers it with a Pong.
func (h *Discv4PeerHarness) ReplyToPing(pkt Discv4Packet) error {
	var ping Discv4Ping
	if err := rlp.DecodeBytes(pkt.Body, &ping); err != nil {
		return err
	}
	to := EndpointOf(pkt.From)
	if ping.From.TCP != 0 {
		to.TCP = ping.From.TCP
	}
	pong := Discv4Pong{
		To:         to,
		ReplyToken: pkt.Hash,
		Expiration: uint64(time.Now().Add(30 * time.Second).Unix()),
	}
	_, err := h.SendPacket(Discv4PongType, &pong)
	if err != nil {
		return err
	}
	return nil
}
