package discovery

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
)

// Discv5Nodes is the decoded discv5 Nodes body.
type Discv5Nodes struct {
	RequestID     []byte
	ResponseCount uint8
	Nodes         []*enr.Record
}

// DecodeNodes decodes a decrypted NODES message body.
func (s *Discv5Conn) DecodeNodes(packet *Discv5OrdinaryPacket) (*Discv5Nodes, error) {
	if packet == nil {
		return nil, errors.New("nil packet")
	}
	if packet.MessageType != Discv5MsgNodes {
		return nil, fmt.Errorf("expected NODES, got msg type 0x%02x", packet.MessageType)
	}
	var nodes Discv5Nodes
	if err := rlp.DecodeBytes(packet.Body, &nodes); err != nil {
		return nil, err
	}
	return &nodes, nil
}
