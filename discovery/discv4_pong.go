package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv4PongType is the discv4 Pong packet type.
const Discv4PongType = 0x02

// Discv4Pong is the discv4 Pong packet body.
type Discv4Pong struct {
	To         Discv4Endpoint
	ReplyToken []byte
	Expiration uint64
	ENRSeq     uint64         `rlp:"optional"`
	Rest       []rlp.RawValue `rlp:"tail"`
}
