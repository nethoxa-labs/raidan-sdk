package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

// NewBlockData is the eth NewBlock message payload.
type NewBlockData struct {
	Block *types.Block
	TD    *big.Int
}
