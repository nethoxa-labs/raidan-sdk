package discovery

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// Discv4Conn owns a UDP socket and a generated local key. It handles the
// signature and hash prefix required by discv4 packets.
type Discv4Conn struct {
	ctx  context.Context
	fd   *net.UDPConn
	key  *ecdsa.PrivateKey
	peer *net.UDPAddr
}

func listenUDPToPeer(peer *net.UDPAddr) (*net.UDPConn, error) {
	if peer == nil || peer.IP == nil {
		return nil, errors.New("discv4 peer has no IP address")
	}
	network, bindIP := "udp6", net.IPv6unspecified
	if peer.IP.To4() != nil {
		network, bindIP = "udp4", net.IPv4zero
	} else if peer.IP.To16() == nil {
		return nil, fmt.Errorf("discv4 peer has invalid IP address %q", peer.IP)
	}
	routeConn, err := net.DialUDP(network, nil, peer)
	if err != nil {
		return nil, fmt.Errorf("resolve local udp source: %w", err)
	}
	local := routeConn.LocalAddr().(*net.UDPAddr)
	_ = routeConn.Close()
	if ip := local.IP; ip != nil && !ip.IsUnspecified() {
		bindIP = ip
	}
	fd, err := net.ListenUDP(network, &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("listen udp on %s: %w", bindIP, err)
	}
	return fd, nil
}

// SameUDPAddr reports whether two UDP addresses share IP, port, and zone.
func SameUDPAddr(left, right *net.UDPAddr) bool {
	return left != nil && right != nil &&
		left.Port == right.Port && left.Zone == right.Zone && left.IP.Equal(right.IP)
}

// ExpectAnyDiscv4Reply waits up to timeout for any discv4 datagram from the
// connection's peer, ignoring datagrams from other sources.
func ExpectAnyDiscv4Reply(conn *Discv4Conn, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining < 100*time.Millisecond {
			remaining = 100 * time.Millisecond
		}
		_, _, from, err := conn.ReadDiscv4(remaining)
		if err == nil && SameUDPAddr(from, conn.PeerAddr()) {
			return true
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			continue
		}
		return false
	}
	return false
}

// DialDiscv4 opens a UDP socket and resolves the peer's UDP endpoint
// from its enode URL.
func DialDiscv4(ctx context.Context, target string) (*Discv4Conn, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	node, err := enode.ParseV4(target)
	if err != nil {
		return nil, fmt.Errorf("parse enode: %w", err)
	}
	if node.IP() == nil || node.IP().IsUnspecified() {
		return nil, errors.New("parse enode: missing peer IP address")
	}
	if node.UDP() < 1 || node.UDP() > 65535 {
		return nil, fmt.Errorf("parse enode: invalid UDP port %d", node.UDP())
	}
	peer := &net.UDPAddr{IP: node.IP(), Port: node.UDP()}
	fd, err := listenUDPToPeer(peer)
	if err != nil {
		return nil, err
	}
	u := &Discv4Conn{ctx: ctx, fd: fd, key: key, peer: peer}
	session.Step(ctx, "[+] Opening discv4 socket to %s", peer)
	return u, nil
}

// Close shuts down the socket.
func (u *Discv4Conn) Close() error {
	return u.fd.Close()
}

// PeerAddr returns the peer UDP address.
func (u *Discv4Conn) PeerAddr() *net.UDPAddr { return u.peer }

// PrivateKey returns the local key used to sign packets.
func (u *Discv4Conn) PrivateKey() *ecdsa.PrivateKey { return u.key }

// LocalAddr returns the local UDP endpoint (useful for building 'from'
// fields in outbound packets).
func (u *Discv4Conn) LocalAddr() *net.UDPAddr {
	return u.fd.LocalAddr().(*net.UDPAddr)
}

// SendRaw writes bytes directly to the peer's UDP port without discv4 framing.
func (u *Discv4Conn) SendRaw(data []byte) error {
	session.ObserveWrite(u.ctx, session.Write{Protocol: "raw", Payload: data, Raw: true})
	_, err := u.fd.WriteToUDP(data, u.peer)
	return err
}

// ReadRaw reads one UDP datagram without interpreting its framing.
func (u *Discv4Conn) ReadRaw(timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, discv4MaxPacketSize)
	n, from, err := readUDPWithContext(u.ctx, u.fd, buffer, timeout)
	if err != nil {
		return nil, nil, err
	}
	return buffer[:n], from, nil
}

// SendDiscv4 signs and sends a discv4 packet.
//
// Wire format: hash(32) || signature(65) || type(1) || packet-data
// Where hash = keccak256(signature || type || packet-data)
// And signature = sign(keccak256(type || packet-data), key)
func (u *Discv4Conn) SendDiscv4(ptype byte, data any) error {
	_, err := u.sendDiscv4Packet(ptype, data)
	return err
}

func (u *Discv4Conn) sendDiscv4Packet(ptype byte, data any) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(data)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	packet, err := signDiscv4Payload(u.key, ptype, encoded)
	if err != nil {
		return nil, err
	}
	session.ObserveWrite(u.ctx, session.Write{Protocol: "discv4", Code: uint64(ptype), Payload: encoded})
	_, err = u.fd.WriteToUDP(packet, u.peer)
	if err != nil {
		return nil, err
	}
	return packet[:32], nil
}

// ReadDiscv4Packet reads one packet with a local timeout. The returned Hash is
// the packet's signed hash prefix, useful when replying to Ping with Pong.
func (u *Discv4Conn) ReadDiscv4Packet(timeout time.Duration) (Discv4Packet, error) {
	packet, from, err := u.ReadRaw(timeout)
	if err != nil {
		return Discv4Packet{}, err
	}
	if len(packet) < discv4HeaderSize+1 {
		return Discv4Packet{}, errors.New("packet too short")
	}
	hash := append([]byte(nil), packet[:32]...)
	body := append([]byte(nil), packet[discv4HeaderSize+1:]...)
	return Discv4Packet{Type: packet[discv4HeaderSize], Body: body, Hash: hash, From: from}, nil
}

// ReadDiscv4 reads one packet with a timeout. Returns (type, body, from, err).
func (u *Discv4Conn) ReadDiscv4(timeout time.Duration) (byte, []byte, *net.UDPAddr, error) {
	pkt, err := u.ReadDiscv4Packet(timeout)
	if err != nil {
		return 0, nil, nil, err
	}
	return pkt.Type, pkt.Body, pkt.From, nil
}
