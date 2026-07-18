package reqresp

import (
	"encoding/binary"
	"fmt"
)

// BlobSidecarsByRootV1 is the BlobSidecarsByRoot v1 protocol ID.
const BlobSidecarsByRootV1 = ProtocolPrefix + "/blob_sidecars_by_root/1/" + Encoding

// MaxSSZBlobIdentifiers is the allocation ceiling for SSZBlobIdentifiers.
const MaxSSZBlobIdentifiers = 1 << 20

// BlobIdentifier is one caller-owned block-root and blob-index pair.
type BlobIdentifier struct {
	BlockRoot Root
	Index     uint64
}

// SSZBlobIdentifiers encodes exactly the caller-supplied identifiers.
func SSZBlobIdentifiers(identifiers []BlobIdentifier) ([]byte, error) {
	if len(identifiers) > MaxSSZBlobIdentifiers {
		return nil, fmt.Errorf("blob identifier count %d exceeds %d", len(identifiers), MaxSSZBlobIdentifiers)
	}
	out := make([]byte, len(identifiers)*40)
	for i, identifier := range identifiers {
		copy(out[i*40:], identifier.BlockRoot[:])
		binary.LittleEndian.PutUint64(out[i*40+32:], identifier.Index)
	}
	return out, nil
}
