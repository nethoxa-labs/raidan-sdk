package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv5RegTopic is a discv5 topic-registration request body.
type Discv5RegTopic struct {
	RequestID []byte
	Topic     []byte
	ENR       []byte
	Ticket    []byte
}

// SendRegTopic sends a RegTopic message from the discv5 topic-registration
// extension.
func (s *Discv5Conn) SendRegTopic(sess *Discv5Session, request Discv5RegTopic) error {
	body, err := rlp.EncodeToBytes(&request)
	if err != nil {
		return err
	}
	return s.SendOrdinary(sess, Discv5MsgRegTopic, body)
}
