package discovery

import (
	"crypto/ecdsa"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	discv4MaxPacketSize = 1280
	discv4HeaderSize    = 32 + 65 // hash + signature
)

// Discv4Packet is a decoded discv4 packet envelope.
type Discv4Packet struct {
	Type byte
	Body []byte
	Hash []byte
	From *net.UDPAddr
}

// Discv4Endpoint is the endpoint tuple embedded in Ping and Pong messages.
type Discv4Endpoint struct {
	IP  net.IP
	UDP uint16
	TCP uint16
}

// EndpointOf converts a UDP address into a discv4 endpoint tuple, advertising
// the same port for UDP and TCP.
func EndpointOf(addr *net.UDPAddr) Discv4Endpoint {
	if addr == nil {
		return Discv4Endpoint{}
	}
	ip := addr.IP.To4()
	if ip == nil {
		ip = addr.IP
	}
	port := uint16(addr.Port)
	return Discv4Endpoint{IP: ip, UDP: port, TCP: port}
}

// PeerEndpoint extracts the advertised endpoint from an enode URL.
func PeerEndpoint(target string) (Discv4Endpoint, error) {
	node, err := enode.ParseV4(target)
	if err != nil {
		return Discv4Endpoint{}, err
	}
	ip := node.IP().To4()
	if ip == nil {
		ip = node.IP()
	}
	return Discv4Endpoint{IP: ip, UDP: uint16(node.UDP()), TCP: uint16(node.TCP())}, nil
}

// BuildSignedDiscv4Packet encodes and signs one discv4 packet.
func BuildSignedDiscv4Packet(key *ecdsa.PrivateKey, packetType byte, value any) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(value)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return signDiscv4Payload(key, packetType, encoded)
}

func signDiscv4Payload(key *ecdsa.PrivateKey, packetType byte, encoded []byte) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("sign: nil private key")
	}
	if 1+len(encoded) > discv4MaxPacketSize-discv4HeaderSize {
		return nil, fmt.Errorf("packet too large: %d", len(encoded))
	}
	body := append([]byte{packetType}, encoded...)
	signature, err := crypto.Sign(crypto.Keccak256(body), key)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	hashInput := make([]byte, 0, len(signature)+len(body))
	hashInput = append(hashInput, signature...)
	hashInput = append(hashInput, body...)
	hash := crypto.Keccak256(hashInput)
	packet := make([]byte, 0, len(hash)+len(hashInput))
	packet = append(packet, hash...)
	packet = append(packet, hashInput...)
	return packet, nil
}
