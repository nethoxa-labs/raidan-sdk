package eth

import (
	"bytes"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"
)

// protoHandshake is the devp2p Hello wire layout. It stays private because
// callers that craft base-protocol Hello messages use rlpx.RawHello directly.
type protoHandshake struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

// validateHelloIdentity applies the same identity binding as go-ethereum's
// high-level devp2p server: Hello.ID must hash to the authenticated target node.
func validateHelloIdentity(node *enode.Node, helloID []byte) error {
	if node == nil {
		return errors.New("target node is nil")
	}
	nodeID := node.ID()
	if !bytes.Equal(crypto.Keccak256(helloID), nodeID[:]) {
		return errors.New("devp2p hello identity does not match authenticated peer")
	}
	return nil
}
