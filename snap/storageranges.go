package snap

import "github.com/ethereum/go-ethereum/common"

// StorageSlot is one entry in a StorageRanges response.
type StorageSlot struct {
	Hash common.Hash
	Data []byte
}

// StorageRangesPacket is the snap/1 StorageRanges response.
type StorageRangesPacket struct {
	ID    uint64
	Slots [][]StorageSlot
	Proof [][]byte
}

// SetRequestID sets the response correlation identifier.
func (p *StorageRangesPacket) SetRequestID(id uint64) { p.ID = id }
