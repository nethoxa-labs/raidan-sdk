package wit

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// NewWitnessHashesMsg is the WIT/1 NewWitnessHashes message code.
const NewWitnessHashesMsg = 0x01

// EncodeNewWitnessHashes encodes parallel witness-hash and block-number lists.
func EncodeNewWitnessHashes(hashes []common.Hash, numbers []uint64) ([]byte, error) {
	return rlp.EncodeToBytes(&struct {
		Hashes  []common.Hash
		Numbers []uint64
	}{Hashes: hashes, Numbers: numbers})
}
