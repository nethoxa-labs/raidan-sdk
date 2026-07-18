package reqresp

import "fmt"

const (
	// BlocksByRootV1 is the BeaconBlocksByRoot v1 protocol ID.
	BlocksByRootV1 = ProtocolPrefix + "/beacon_blocks_by_root/1/" + Encoding
	// BlocksByRootV2 is the BeaconBlocksByRoot v2 protocol ID.
	BlocksByRootV2 = ProtocolPrefix + "/beacon_blocks_by_root/2/" + Encoding
)

// Root is one caller-owned SSZ root.
type Root [32]byte

// MaxSSZRootListItems is the allocation ceiling for SSZRootList.
const MaxSSZRootListItems = 1 << 20

// SSZRootList encodes exactly the caller-supplied SSZ roots.
func SSZRootList(roots []Root) ([]byte, error) {
	if len(roots) > MaxSSZRootListItems {
		return nil, fmt.Errorf("root count %d exceeds %d", len(roots), MaxSSZRootListItems)
	}
	out := make([]byte, len(roots)*len(Root{}))
	for i := range roots {
		copy(out[i*len(Root{}):], roots[i][:])
	}
	return out, nil
}
