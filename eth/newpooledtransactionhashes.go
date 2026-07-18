package eth

import "github.com/ethereum/go-ethereum/common"

// NewPooledTxHashes68 is the ETH/68 NewPooledTransactionHashes message.
type NewPooledTxHashes68 struct {
	Types  []byte
	Sizes  []uint32
	Hashes []common.Hash
}

// NewPooledTxHashes72 is the ETH/72 NewPooledTransactionHashes message.
type NewPooledTxHashes72 struct {
	Types  []byte
	Sizes  []uint32
	Hashes []common.Hash
	Cells  []byte
}
