package utils

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const deterministicSeed = "raidan-sdk-deterministic-key-v1"

// ParticipantIdentity is one case-scoped protocol identity. The same key is
// used for EL and CL transports inside one case. A new case must use a new
// identity so remote peer state cannot affect a later case.
type ParticipantIdentity struct {
	key *ecdsa.PrivateKey
}

// NewParticipantIdentity creates a cryptographically random case identity.
func NewParticipantIdentity() (*ParticipantIdentity, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate participant identity: %w", err)
	}
	return &ParticipantIdentity{key: key}, nil
}

// ParticipantIdentityFromKey creates an identity from explicit secp256k1 key
// material. The key is copied so callers cannot mutate shared identity state.
func ParticipantIdentityFromKey(key *ecdsa.PrivateKey) (*ParticipantIdentity, error) {
	if key == nil {
		return nil, errors.New("participant identity key is nil")
	}
	copyKey, err := crypto.ToECDSA(crypto.FromECDSA(key))
	if err != nil {
		return nil, fmt.Errorf("copy participant identity: %w", err)
	}
	return &ParticipantIdentity{key: copyKey}, nil
}

// ELKey returns an independent copy of the identity key for EL transports.
func (identity *ParticipantIdentity) ELKey() (*ecdsa.PrivateKey, error) {
	if identity == nil || identity.key == nil {
		return nil, errors.New("participant identity is nil")
	}
	key, err := crypto.ToECDSA(crypto.FromECDSA(identity.key))
	if err != nil {
		return nil, fmt.Errorf("copy participant identity: %w", err)
	}
	return key, nil
}

// CLKey returns the identity in libp2p's secp256k1 representation.
func (identity *ParticipantIdentity) CLKey() (libp2pcrypto.PrivKey, error) {
	key, err := identity.ELKey()
	if err != nil {
		return nil, err
	}
	return libp2pcrypto.UnmarshalSecp256k1PrivateKey(crypto.FromECDSA(key))
}

// ELPeerIdentities returns the exact discovery node ID and RLPx public key.
func (identity *ParticipantIdentity) ELPeerIdentities() ([]string, error) {
	key, err := identity.ELKey()
	if err != nil {
		return nil, err
	}
	nodeID := fmt.Sprintf("%x", crypto.Keccak256(crypto.FromECDSAPub(&key.PublicKey)[1:]))
	publicKey := fmt.Sprintf("%x", crypto.FromECDSAPub(&key.PublicKey)[1:])
	return []string{nodeID, publicKey}, nil
}

// CLPeerIdentities returns the exact libp2p peer ID and discv5 node ID.
func (identity *ParticipantIdentity) CLPeerIdentities() ([]string, error) {
	key, err := identity.CLKey()
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

// DeterministicKey derives the same secp256k1 key for the same label. It is
// stateless; callers that need an ephemeral identity should use
// crypto.GenerateKey directly.
func DeterministicKey(label string) (*ecdsa.PrivateKey, error) {
	if label == "" {
		return nil, errors.New("derive deterministic key: label is empty")
	}
	input := make([]byte, 0, len(deterministicSeed)+1+len(label)+4)
	input = append(input, deterministicSeed...)
	input = append(input, 0)
	input = append(input, label...)
	input = append(input, 0, 0, 0, 0)
	for attempt := uint32(0); attempt < 1024; attempt++ {
		binary.BigEndian.PutUint32(input[len(input)-4:], attempt)
		sum := sha256.Sum256(input)
		key, err := crypto.ToECDSA(sum[:])
		if err == nil {
			return key, nil
		}
	}
	return nil, errors.New("derive deterministic key: exhausted attempts")
}
