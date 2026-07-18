package eth

import (
	"github.com/ethereum/go-ethereum/core/types"
)

// ReceiptsPacket is the eth/66+ Receipts response wrapper.
type ReceiptsPacket struct {
	RequestID uint64
	Receipts  []types.Receipts
}

// ReceiptsPacket70 is the eth/70+ Receipts response wrapper.
type ReceiptsPacket70 struct {
	RequestID           uint64
	LastBlockIncomplete bool
	Receipts            []types.Receipts
}
