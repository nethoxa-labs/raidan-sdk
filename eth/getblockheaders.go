package eth

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// GetBlockHeadersRequest is the nested eth/66+ block-header query.
type GetBlockHeadersRequest struct {
	Origin  HashOrNumber
	Amount  uint64
	Skip    uint64
	Reverse bool
}

// GetBlockHeadersPacket wraps a block-header query with its request ID.
// Its wire shape is [request-id, [origin, amount, skip, reverse]].
type GetBlockHeadersPacket struct {
	RequestID uint64
	*GetBlockHeadersRequest
}

// HashOrNumber is the origin selector for GetBlockHeaders.
type HashOrNumber struct {
	Hash   common.Hash
	Number uint64
}

// EncodeRLP writes either the hash or the block number, per devp2p.
func (h *HashOrNumber) EncodeRLP(w io.Writer) error {
	if h.Hash == (common.Hash{}) {
		return rlp.Encode(w, h.Number)
	}
	if h.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", h.Hash, h.Number)
	}
	return rlp.Encode(w, h.Hash)
}

// DecodeRLP reads either a 32-byte hash or an unsigned block number.
func (h *HashOrNumber) DecodeRLP(stream *rlp.Stream) error {
	_, size, err := stream.Kind()
	switch {
	case err != nil:
		return err
	case size == common.HashLength:
		h.Number = 0
		return stream.Decode(&h.Hash)
	case size <= 8:
		h.Hash = common.Hash{}
		return stream.Decode(&h.Number)
	default:
		return fmt.Errorf("invalid input size %d for origin", size)
	}
}
