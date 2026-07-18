package discovery

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// BuildSignedENR signs the supplied, pre-encoded key/value pairs with key.
// No sorting or validation is performed.
func BuildSignedENR(sequence uint64, pairs []rlp.RawValue, key *ecdsa.PrivateKey) (rlp.RawValue, error) {
	signature, err := SignENRContent(sequence, pairs, key)
	if err != nil {
		return nil, err
	}
	return BuildENRWithSignature(sequence, pairs, signature)
}

// BuildV4ENR builds a minimal identity-v4 ENR whose public key and signature
// are both derived from key.
func BuildV4ENR(sequence uint64, key *ecdsa.PrivateKey) (rlp.RawValue, error) {
	if key == nil {
		return nil, fmt.Errorf("build ENR: nil signing key")
	}
	publicKey := crypto.CompressPubkey(&key.PublicKey)
	identity, err := rlp.EncodeToBytes("v4")
	if err != nil {
		return nil, fmt.Errorf("encode ENR identity: %w", err)
	}
	publicKeyRLP, err := rlp.EncodeToBytes(publicKey)
	if err != nil {
		return nil, fmt.Errorf("encode ENR public key: %w", err)
	}
	pairs := BuildENRKeyValue("id", identity)
	pairs = append(pairs, BuildENRKeyValue("secp256k1", publicKeyRLP)...)
	return BuildSignedENR(sequence, pairs, key)
}

// BuildENRWithSignature assembles an ENR with a caller-supplied 64-byte r||s
// signature without deriving or validating it.
func BuildENRWithSignature(sequence uint64, pairs []rlp.RawValue, signature []byte) (rlp.RawValue, error) {
	if len(signature) != 64 {
		return nil, fmt.Errorf("ENR signature must be 64 bytes, got %d", len(signature))
	}
	sequenceRLP := rlp.AppendUint64(nil, sequence)
	signatureRLP, err := rlp.EncodeToBytes(signature)
	if err != nil {
		return nil, err
	}
	record := make([]rlp.RawValue, 0, 2+len(pairs))
	record = append(record, signatureRLP, sequenceRLP)
	record = append(record, pairs...)
	return rlp.EncodeToBytes(record)
}

// SignENRContent signs the identity-v4 hash of sequence and pairs and returns
// the raw 64-byte r||s signature.
func SignENRContent(sequence uint64, pairs []rlp.RawValue, key *ecdsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("sign ENR: nil key")
	}
	sequenceRLP := rlp.AppendUint64(nil, sequence)
	content := make([]rlp.RawValue, 0, 1+len(pairs))
	content = append(content, sequenceRLP)
	content = append(content, pairs...)
	encoded, err := rlp.EncodeToBytes(content)
	if err != nil {
		return nil, err
	}
	signature, err := crypto.Sign(crypto.Keccak256(encoded), key)
	if err != nil {
		return nil, err
	}
	return signature[:64], nil
}
