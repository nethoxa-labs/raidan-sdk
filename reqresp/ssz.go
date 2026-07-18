package reqresp

import "encoding/binary"

// SSZUint64 returns the SSZ encoding of value.
func SSZUint64(value uint64) []byte {
	var out [8]byte
	binary.LittleEndian.PutUint64(out[:], value)
	return out[:]
}

// SSZTwoUint64Request returns two consecutive SSZ uint64 fields.
func SSZTwoUint64Request(first, second uint64) []byte {
	out := make([]byte, 16)
	binary.LittleEndian.PutUint64(out[0:8], first)
	binary.LittleEndian.PutUint64(out[8:16], second)
	return out
}
