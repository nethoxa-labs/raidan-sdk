package eth

import "github.com/ethereum/go-ethereum/common"

// GetBlockAccessListsPacket is the eth/71 GetBlockAccessLists request.
type GetBlockAccessListsPacket struct {
	RequestID uint64
	Hashes    []common.Hash
}
