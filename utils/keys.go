package utils

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

const deterministicSeed = "raidan-sdk-deterministic-key-v1"

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
