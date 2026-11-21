package wit

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// GetWitnessMsg is the WIT/1 GetWitness message code.
const GetWitnessMsg = 0x02

// PageRequest identifies one page of a witness.
type PageRequest struct {
	Hash common.Hash
	Page uint64
}

// EncodeGetWitness encodes a GetWitness payload. WIT/1 wraps the page list in
// an additional list, yielding [request-id, [pages]].
func EncodeGetWitness(requestID uint64, pages []PageRequest) ([]byte, error) {
	return rlp.EncodeToBytes([]any{requestID, []any{pages}})
}
