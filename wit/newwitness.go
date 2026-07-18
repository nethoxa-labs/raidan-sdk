package wit

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// NewWitnessMsg is the WIT/1 NewWitness message code.
const NewWitnessMsg = 0x00

type witnessPayload struct {
	Context *types.Header
	Headers []*types.Header
	State   [][]byte
}

// EncodeNewWitness encodes a NewWitness payload.
func EncodeNewWitness(context *types.Header, headers []*types.Header, state [][]byte) ([]byte, error) {
	return rlp.EncodeToBytes(&struct {
		Witness witnessPayload
	}{Witness: witnessPayload{
		Context: context,
		Headers: headers,
		State:   state,
	}})
}
