package discovery

import "github.com/ethereum/go-ethereum/rlp"

// Discv4ENRResponseType is the discv4 ENRResponse packet type.
const Discv4ENRResponseType = 0x06

// Discv4ENRResponse is the discv4 ENRResponse packet body.
type Discv4ENRResponse struct {
	RequestHash [32]byte
	ENR         rlp.RawValue
	Rest        []rlp.RawValue `rlp:"tail"`
}
