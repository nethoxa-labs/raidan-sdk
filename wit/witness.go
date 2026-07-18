package wit

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// WitnessMsg is the WIT/1 Witness message code.
const WitnessMsg = 0x03

// PageResponse carries one witness page.
type PageResponse struct {
	Data       []byte
	Hash       common.Hash
	Page       uint64
	TotalPages uint64
}

// EncodeWitness encodes a Witness response payload.
func EncodeWitness(requestID uint64, responses []PageResponse) ([]byte, error) {
	return rlp.EncodeToBytes([]any{requestID, responses})
}
