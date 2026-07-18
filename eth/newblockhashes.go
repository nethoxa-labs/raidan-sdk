package eth

import "github.com/ethereum/go-ethereum/common"

// NewBlockHashEntry is one entry in a NewBlockHashes message.
type NewBlockHashEntry struct {
	Hash   common.Hash
	Number uint64
}
