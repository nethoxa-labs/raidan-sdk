package eth

import "github.com/ethereum/go-ethereum/common"

// GetBlockBodiesPacket is the eth/66+ GetBlockBodies request.
type GetBlockBodiesPacket struct {
	RequestID uint64
	Hashes    []common.Hash
}
