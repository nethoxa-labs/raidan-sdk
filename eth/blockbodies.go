package eth

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// BlockBody is an ETH block body whose transactions remain pre-encoded.
type BlockBody struct {
	Txs         []rlp.RawValue
	Uncles      []*types.Header
	Withdrawals []*types.Withdrawal `rlp:"optional"`
}

// BlockBodiesPacket is the eth/66+ BlockBodies response wrapper.
type BlockBodiesPacket struct {
	RequestID uint64
	Bodies    []*BlockBody
}
