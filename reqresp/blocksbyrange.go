package reqresp

import "encoding/binary"

const (
	// BlocksByRangeV1 is the BeaconBlocksByRange v1 protocol ID.
	BlocksByRangeV1 = ProtocolPrefix + "/beacon_blocks_by_range/1/" + Encoding
	// BlocksByRangeV2 is the BeaconBlocksByRange v2 protocol ID.
	BlocksByRangeV2 = ProtocolPrefix + "/beacon_blocks_by_range/2/" + Encoding
)

// SSZBlocksByRangeRequest encodes start slot, count, and step.
func SSZBlocksByRangeRequest(startSlot, count, step uint64) []byte {
	out := make([]byte, 24)
	binary.LittleEndian.PutUint64(out[0:8], startSlot)
	binary.LittleEndian.PutUint64(out[8:16], count)
	binary.LittleEndian.PutUint64(out[16:24], step)
	return out
}
