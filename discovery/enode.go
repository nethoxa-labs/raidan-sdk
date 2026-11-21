package discovery

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// PeerPublicKey returns the peer's static secp256k1 public key from an enode URL.
func PeerPublicKey(target string) (*ecdsa.PublicKey, error) {
	node, err := enode.ParseV4(target)
	if err != nil {
		return nil, err
	}
	return node.Pubkey(), nil
}

// Discv4PublicKeyBytes returns the 64-byte X||Y public-key representation used
// as a discv4 node identifier.
func Discv4PublicKeyBytes(target string) ([64]byte, error) {
	var result [64]byte
	publicKey, err := PeerPublicKey(target)
	if err != nil {
		return result, err
	}
	copy(result[:], crypto.FromECDSAPub(publicKey)[1:])
	return result, nil
}
