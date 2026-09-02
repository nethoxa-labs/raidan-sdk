package reqresp

import (
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/nethoxa-labs/raidan-sdk/utils"
)

// CLKey returns the case identity in libp2p's secp256k1 representation.
func CLKey(identity *utils.ParticipantIdentity) (libp2pcrypto.PrivKey, error) {
	key, err := identity.ELKey()
	if err != nil {
		return nil, err
	}
	return libp2pcrypto.UnmarshalSecp256k1PrivateKey(crypto.FromECDSA(key))
}

// CLPeerIdentities returns the exact libp2p peer ID and discv5 node ID.
func CLPeerIdentities(identity *utils.ParticipantIdentity) ([]string, error) {
	key, err := CLKey(identity)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("derive participant peer ID: %w", err)
	}
	secp256k1Key, err := identity.ELKey()
	if err != nil {
		return nil, err
	}
	nodeID := fmt.Sprintf("%x", crypto.Keccak256(crypto.FromECDSAPub(&secp256k1Key.PublicKey)[1:]))
	return []string{id.String(), nodeID}, nil
}
