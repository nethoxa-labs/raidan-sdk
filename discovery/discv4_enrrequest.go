package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv4ENRRequestType is the discv4 ENRRequest packet type.
const Discv4ENRRequestType = 0x05

// Discv4ENRRequest is the discv4 ENRRequest packet body.
type Discv4ENRRequest struct {
	Expiration uint64
	Rest       []rlp.RawValue `rlp:"tail"`
}
