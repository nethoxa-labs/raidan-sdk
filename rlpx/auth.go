package rlpx

// RLPx EIP-8 auth body primitives used by the internal handshake.

import (
	"github.com/ethereum/go-ethereum/rlp"
)

type authMsgV4 struct {
	Signature       [65]byte
	InitiatorPubkey [64]byte
	Nonce           [32]byte
	Version         uint

	Rest []rlp.RawValue `rlp:"tail"`
}

func xorBytesAuth(a, b []byte) []byte {
	if len(a) != len(b) {
		panic("xorBytesAuth: length mismatch")
	}
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i]
	}
	return out
}

const rlpxAuthEciesOverhead = 65 + 16 + 32
