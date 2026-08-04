package discovery

import (
	"crypto/ecdsa"
	"fmt"
	"net"
	"sort"

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

// BuildV4ENRWithValues builds a signed identity-v4 ENR and adds caller-owned
// RLP-compatible values in canonical key order.
func BuildV4ENRWithValues(sequence uint64, key *ecdsa.PrivateKey, values map[string]any) (rlp.RawValue, error) {
	if key == nil {
		return nil, fmt.Errorf("build ENR: nil signing key")
	}
	entries := make(map[string]any, len(values)+2)
	entries["id"] = "v4"
	entries["secp256k1"] = crypto.CompressPubkey(&key.PublicKey)
	for name, value := range values {
		if name == "id" || name == "secp256k1" {
			return nil, fmt.Errorf("build ENR: reserved key %q", name)
		}
		entries[name] = value
	}
	keys := make([]string, 0, len(entries))
	for name := range entries {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	pairs := make([]rlp.RawValue, 0, len(keys)*2)
	for _, name := range keys {
		encoded, err := rlp.EncodeToBytes(entries[name])
		if err != nil {
			return nil, fmt.Errorf("encode ENR %s: %w", name, err)
		}
		pairs = append(pairs, BuildENRKeyValue(name, encoded)...)
	}
	return BuildSignedENR(sequence, pairs, key)
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

// BuildV4ENRWithEndpoint builds a signed identity-v4 ENR with an IP address
// and UDP endpoint. It includes TCP when tcp is non-zero.
func BuildV4ENRWithEndpoint(sequence uint64, key *ecdsa.PrivateKey, ip net.IP, udp, tcp uint16) (rlp.RawValue, error) {
	if key == nil {
		return nil, fmt.Errorf("build ENR: nil signing key")
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("build ENR: endpoint is not IPv4")
	}
	if udp == 0 {
		return nil, fmt.Errorf("build ENR: UDP port is zero")
	}
	identity, err := rlp.EncodeToBytes("v4")
	if err != nil {
		return nil, fmt.Errorf("encode ENR identity: %w", err)
	}
	ipValue, err := rlp.EncodeToBytes([]byte(ipv4))
	if err != nil {
		return nil, fmt.Errorf("encode ENR IPv4 address: %w", err)
	}
	publicKey, err := rlp.EncodeToBytes(crypto.CompressPubkey(&key.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("encode ENR public key: %w", err)
	}
	udpValue, err := rlp.EncodeToBytes(udp)
	if err != nil {
		return nil, fmt.Errorf("encode ENR UDP port: %w", err)
	}
	pairs := BuildENRKeyValue("id", identity)
	pairs = append(pairs, BuildENRKeyValue("ip", ipValue)...)
	pairs = append(pairs, BuildENRKeyValue("secp256k1", publicKey)...)
	if tcp != 0 {
		tcpValue, encodeErr := rlp.EncodeToBytes(tcp)
		if encodeErr != nil {
			return nil, fmt.Errorf("encode ENR TCP port: %w", encodeErr)
		}
		pairs = append(pairs, BuildENRKeyValue("tcp", tcpValue)...)
	}
	pairs = append(pairs, BuildENRKeyValue("udp", udpValue)...)
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
