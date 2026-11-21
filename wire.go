package sdk

import "encoding/binary"

// EncodeRLPBytes returns the canonical RLP byte-string encoding of payload.
func EncodeRLPBytes(payload []byte) []byte {
	if len(payload) == 1 && payload[0] < 0x80 {
		return append([]byte(nil), payload...)
	}
	if len(payload) <= 55 {
		out := make([]byte, 1+len(payload))
		out[0] = 0x80 + byte(len(payload))
		copy(out[1:], payload)
		return out
	}
	length := minimalBigEndian(uint64(len(payload)))
	out := make([]byte, 1+len(length)+len(payload))
	out[0] = 0xb7 + byte(len(length))
	copy(out[1:], length)
	copy(out[1+len(length):], payload)
	return out
}

// EncodeRLPList wraps caller-supplied, pre-encoded RLP items in one list.
func EncodeRLPList(items ...[]byte) []byte {
	payloadSize := 0
	for _, item := range items {
		payloadSize += len(item)
	}
	out := make([]byte, 0, payloadSize+9)
	if payloadSize <= 55 {
		out = append(out, 0xc0+byte(payloadSize))
	} else {
		length := minimalBigEndian(uint64(payloadSize))
		out = append(out, 0xf7+byte(len(length)))
		out = append(out, length...)
	}
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

func minimalBigEndian(value uint64) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	first := 0
	for first < len(buffer)-1 && buffer[first] == 0 {
		first++
	}
	return buffer[first:]
}
