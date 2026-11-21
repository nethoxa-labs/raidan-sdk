package snap

import "github.com/ethereum/go-ethereum/common"

// GetTrieNodesPacket is the snap/1 GetTrieNodes request.
type GetTrieNodesPacket struct {
	ID    uint64
	Root  common.Hash
	Paths [][][]byte
	Bytes uint64
}
