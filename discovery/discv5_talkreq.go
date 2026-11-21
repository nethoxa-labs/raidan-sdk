package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv5TalkRequest is a discv5 TalkReq body.
type Discv5TalkRequest struct {
	RequestID []byte
	Protocol  []byte
	Message   []byte
}

// SendTalkRequest sends a TalkReq message.
func (s *Discv5Conn) SendTalkRequest(sess *Discv5Session, request Discv5TalkRequest) error {
	body, err := rlp.EncodeToBytes(&request)
	if err != nil {
		return err
	}
	return s.SendOrdinary(sess, Discv5MsgTalkRequest, body)
}
