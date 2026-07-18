package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv4PingType is the discv4 Ping packet type.
const Discv4PingType = 0x01

// Discv4Ping is the discv4 Ping packet body.
type Discv4Ping struct {
	Version    uint64
	From       Discv4Endpoint
	To         Discv4Endpoint
	Expiration uint64
	ENRSeq     uint64         `rlp:"optional"`
	Rest       []rlp.RawValue `rlp:"tail"`
}
