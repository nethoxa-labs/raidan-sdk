package eth

import "github.com/ethereum/go-ethereum/common"

// GetReceiptsPacket is the eth/66+ GetReceipts request.
type GetReceiptsPacket struct {
	RequestID uint64
	Hashes    []common.Hash
}

// GetReceiptsPacket70 is the eth/70+ GetReceipts request.
type GetReceiptsPacket70 struct {
	RequestID              uint64
	FirstBlockReceiptIndex uint64
	Hashes                 []common.Hash
}
