package discovery

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// Discv5Conn holds the live UDP connection plus the static crypto state.
type Discv5Conn struct {
	ctx        context.Context
	fd         *net.UDPConn
	peerAddr   *net.UDPAddr
	peerNodeID enode.ID
	peerStatic *ecdsa.PublicKey
	ourPriv    *ecdsa.PrivateKey
	ourNodeID  enode.ID
}

// DialDiscv5 opens a UDP socket aimed at the discv5 endpoint of the
// given enode and generates a fresh static keypair for our side.
func DialDiscv5(ctx context.Context, target string) (*Discv5Conn, error) {
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
	priv, err := crypto.GenerateKey()
	if err != nil {
		_ = fd.Close()
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &Discv5Conn{
		ctx:        ctx,
		fd:         fd,
		peerAddr:   peer,
		peerNodeID: node.ID(),
		peerStatic: node.Pubkey(),
		ourPriv:    priv,
		ourNodeID:  enode.PubkeyToIDV4(&priv.PublicKey),
	}, nil
}

// Close releases the UDP socket.
func (s *Discv5Conn) Close() error { return s.fd.Close() }

// PeerAddr returns the peer UDP address.
func (s *Discv5Conn) PeerAddr() *net.UDPAddr { return s.peerAddr }

// PeerNodeID returns the recipient's enode-id (used as masking key seed).
func (s *Discv5Conn) PeerNodeID() enode.ID { return s.peerNodeID }

// PeerPublicKey returns an independent copy of the recipient's static key.
func (s *Discv5Conn) PeerPublicKey() *ecdsa.PublicKey {
	if s.peerStatic == nil {
		return nil
	}
	return &ecdsa.PublicKey{
		Curve: s.peerStatic.Curve,
		X:     new(big.Int).Set(s.peerStatic.X),
		Y:     new(big.Int).Set(s.peerStatic.Y),
	}
}

// LocalNodeID returns the node ID derived from the local keypair.
func (s *Discv5Conn) LocalNodeID() enode.ID { return s.ourNodeID }

// PrivateKey returns the local static key used by the handshake.
func (s *Discv5Conn) PrivateKey() *ecdsa.PrivateKey { return s.ourPriv }

// ReadRaw reads one UDP datagram without interpreting its framing.
func (s *Discv5Conn) ReadRaw(timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, discv4MaxPacketSize)
	n, from, err := readUDPWithContext(s.ctx, s.fd, buffer, timeout)
	if err != nil {
		return nil, nil, err
	}
	return buffer[:n], from, nil
}
