package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv5Ping is a discv5 Ping body. RequestID may be empty when probing
// request-ID validation.
type Discv5Ping struct {
	RequestID []byte
	ENRSeq    uint64
}

// SendPing sends a Ping.
func (s *Discv5Conn) SendPing(sess *Discv5Session, ping Discv5Ping) error {
	body, err := rlp.EncodeToBytes(&ping)
	if err != nil {
		return err
	}
	return s.SendOrdinary(sess, Discv5MsgPing, body)
}
