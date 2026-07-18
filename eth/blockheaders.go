package eth

import "github.com/ethereum/go-ethereum/core/types"

// BlockHeadersPacket is the eth/66+ BlockHeaders response wrapper.
type BlockHeadersPacket struct {
	RequestID uint64
	Headers   []*types.Header
}
