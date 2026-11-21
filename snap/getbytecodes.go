package snap

import "github.com/ethereum/go-ethereum/common"

// GetByteCodesPacket is the snap/1 GetByteCodes request.
type GetByteCodesPacket struct {
	ID     uint64
	Hashes []common.Hash
	Bytes  uint64
}
