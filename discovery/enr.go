package discovery

// ENR wire-format helpers per https://github.com/ethereum/devp2p/blob/master/enr.md#record-structure (EIP-778):
//
//	record = [signature, seq, k_1, v_1, k_2, v_2, ...]
//	signature = sign(keccak256(rlp([seq, k_1, v_1, ...])), secp256k1 key)
//
// Max encoded size: 300 bytes.

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/rlp"
)

// BuildENRKeyValue turns a key and an encoded value into two RLP items in the
// canonical order used inside ENR records.
func BuildENRKeyValue(key string, value rlp.RawValue) []rlp.RawValue {
	kb, _ := rlp.EncodeToBytes([]byte(key))
	return []rlp.RawValue{kb, value}
}

// Uint16Bytes returns the two-byte, big-endian encoding of value. ENR port
// fields use this representation.
func Uint16Bytes(value uint16) []byte {
	encoded := make([]byte, 2)
	binary.BigEndian.PutUint16(encoded, value)
	return encoded
}
