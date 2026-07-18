package snap

import (
	"github.com/ethereum/go-ethereum/rlp"
)

// BlockAccessListsPacket is the snap/2 BlockAccessLists response.
type BlockAccessListsPacket struct {
	ID          uint64
	AccessLists []rlp.RawValue
}

// SetRequestID sets the response correlation identifier.
func (p *BlockAccessListsPacket) SetRequestID(id uint64) { p.ID = id }
