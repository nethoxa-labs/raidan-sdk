package snap

import "github.com/ethereum/go-ethereum/common"

// GetAccountRangePacket is the snap/1 GetAccountRange request.
type GetAccountRangePacket struct {
	ID     uint64
	Root   common.Hash
	Origin common.Hash
	Limit  common.Hash
	Bytes  uint64
}
