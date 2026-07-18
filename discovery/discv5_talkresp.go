package discovery

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
)

// Discv5TalkResponse is the decoded discv5 TalkResp body.
type Discv5TalkResponse struct {
	RequestID []byte
	Message   []byte
}

// DecodeTalkResponse decodes a decrypted TALKRESP message body.
func (s *Discv5Conn) DecodeTalkResponse(packet *Discv5OrdinaryPacket) (*Discv5TalkResponse, error) {
	if packet == nil {
		return nil, errors.New("nil packet")
	}
	if packet.MessageType != Discv5MsgTalkResponse {
		return nil, fmt.Errorf("expected TALKRESP, got msg type 0x%02x", packet.MessageType)
	}
	var response Discv5TalkResponse
	if err := rlp.DecodeBytes(packet.Body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
