package eth

import "github.com/ethereum/go-ethereum/common"

// GetCellsPacket is the eth/72 GetCells request.
type GetCellsPacket struct {
	RequestID uint64
	Hashes    []common.Hash
	Mask      CustodyBitmap
}
