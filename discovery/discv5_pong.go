package discovery

import (
	"errors"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/rlp"
)

// Discv5Pong is the decoded discv5 Pong body.
type Discv5Pong struct {
	RequestID []byte
	ENRSeq    uint64
	ToIP      net.IP
	ToPort    uint16
}

// DecodePong decodes a decrypted PONG message body.
func (s *Discv5Conn) DecodePong(packet *Discv5OrdinaryPacket) (*Discv5Pong, error) {
	if packet == nil {
		return nil, errors.New("nil packet")
	}
	if packet.MessageType != Discv5MsgPong {
		return nil, fmt.Errorf("expected PONG, got msg type 0x%02x", packet.MessageType)
	}
	var pong Discv5Pong
	if err := rlp.DecodeBytes(packet.Body, &pong); err != nil {
		return nil, err
	}
	return &pong, nil
}
