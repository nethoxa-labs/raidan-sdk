package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// CellsPacket is the canonical eth/72 Cells response.
type CellsPacket struct {
	RequestID uint64
	Hashes    []common.Hash
	Cells     [][]rlp.RawValue
	Mask      CustodyBitmap
}
