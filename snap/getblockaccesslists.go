package snap

import "github.com/ethereum/go-ethereum/common"

// GetBlockAccessListsPacket is the snap/2 GetBlockAccessLists request.
type GetBlockAccessListsPacket struct {
	ID     uint64
	Hashes []common.Hash
	Bytes  uint64
}
