package discovery

// Secp256k1 curve helpers and ENR signing utilities.

import "math/big"

// Uint256Bytes returns the low 256 bits of a non-negative v as 32 big-endian
// bytes. It panics for negative values.
func Uint256Bytes(v *big.Int) []byte {
	if v.Sign() < 0 {
		panic("negative integer cannot be encoded as uint256")
	}
	b := v.Bytes()
	if len(b) > 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}
