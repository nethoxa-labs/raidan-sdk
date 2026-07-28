package utils

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

const deterministicSeed = "raidan-sdk-deterministic-key-v1"

// raidanParticipantPrivateKeyHex is public, test-only key material used by
// ordinary Raidan protocol deliveries. It must never be used for a production
// Ethereum account or validator.
const raidanParticipantPrivateKeyHex = "4c2b1f9aa8d146e8bf2f82f9d8cb43c97a074e9d57e22e71669949f5d9c71231"

// RaidanParticipantELNodeID is the discovery/RLPx node ID derived from the
// canonical participant key. It is public protocol identity, not secret key
// material, and is exposed so topology hooks can classify Raidan traffic.
const RaidanParticipantELNodeID = "a730ede5a8d042544817f72a1b6d388691ac43593e2d0de60e8c46f0dc1b3302"

// RaidanParticipantELPublicKey is the 64-byte uncompressed secp256k1 public
// key (without the 0x04 prefix) used by RLPx Hello messages.
const RaidanParticipantELPublicKey = "82e3296353f492daced63f02a1e0d1d11134360bf1b6aa4f4a143306be13a4a4d47294b89292275674c8c062135492bb5e7a295b83e5d0d6fd364f4bc9f34898"

// RaidanParticipantCLPeerID is the libp2p peer ID derived from the canonical
// participant key and used by consensus req/resp and gossip traffic.
const RaidanParticipantCLPeerID = "16Uiu2HAm4EbtCqGM6541nhbjPfTtqXrKYXjBQnAuS37om4inVeNX"

// RaidanParticipantKey returns the canonical secp256k1 identity for RLPx and
// discovery traffic. The key is reparsed per call so callers never share
// mutable private-key state.
func RaidanParticipantKey() (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(raidanParticipantPrivateKeyHex)
}

// RaidanParticipantLibp2pKey returns the same canonical secp256k1 material in
// libp2p's identity representation for consensus req/resp and gossip traffic.
func RaidanParticipantLibp2pKey() (libp2pcrypto.PrivKey, error) {
	key, err := RaidanParticipantKey()
	if err != nil {
		return nil, err
	}
	return libp2pcrypto.UnmarshalSecp256k1PrivateKey(crypto.FromECDSA(key))
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
