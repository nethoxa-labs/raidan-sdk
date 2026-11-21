package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv5FindNode is a discv5 FindNode body.
type Discv5FindNode struct {
	RequestID []byte
	Distances []uint64
}

// SendFindNode sends a FindNode.
func (s *Discv5Conn) SendFindNode(sess *Discv5Session, request Discv5FindNode) error {
	body, err := rlp.EncodeToBytes(&request)
	if err != nil {
		return err
	}
	return s.SendOrdinary(sess, Discv5MsgFindNode, body)
}
