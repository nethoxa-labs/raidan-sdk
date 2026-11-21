package snap

import "github.com/ethereum/go-ethereum/common"

// GetStorageRangesPacket is the snap/1 GetStorageRanges request.
type GetStorageRangesPacket struct {
	ID       uint64
	Root     common.Hash
	Accounts []common.Hash
	Origin   []byte
	Limit    []byte
	Bytes    uint64
}
