package discovery

import (
	"crypto/rand"

	"github.com/ethereum/go-ethereum/rlp"
)

// Discv4FindNodeType is the discv4 FindNode packet type.
const Discv4FindNodeType = 0x03

// Discv4FindNode is the discv4 FindNode packet body.
type Discv4FindNode struct {
	Target     [64]byte
	Expiration uint64
	Rest       []rlp.RawValue `rlp:"tail"`
}

// RandomFindNodeTarget returns a cryptographically random 64-byte lookup target.
func RandomFindNodeTarget() ([64]byte, error) {
	var target [64]byte
	_, err := rand.Read(target[:])
	return target, err
}
