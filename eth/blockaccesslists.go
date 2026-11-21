package eth

import (
	"github.com/ethereum/go-ethereum/rlp"
)

// BlockAccessListsPacket is the eth/71 BlockAccessLists response.
type BlockAccessListsPacket struct {
	RequestID uint64
	List      []rlp.RawValue
}
