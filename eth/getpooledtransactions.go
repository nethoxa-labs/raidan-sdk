package eth

import "github.com/ethereum/go-ethereum/common"

// GetPooledTxPacket is the eth/66+ GetPooledTransactions request.
type GetPooledTxPacket struct {
	RequestID uint64
	Hashes    []common.Hash
}
