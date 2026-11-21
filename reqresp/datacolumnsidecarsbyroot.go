package reqresp

import (
	"encoding/binary"
	"fmt"
	"math"
)

// DataColumnsByRootV1 is the DataColumnSidecarsByRoot v1 protocol ID.
const DataColumnsByRootV1 = ProtocolPrefix + "/data_column_sidecars_by_root/1/" + Encoding

const (
	// MaxDataColumnRootIdentifiers caps identifiers in one generated request.
	MaxDataColumnRootIdentifiers = 128
	// MaxDataColumnsPerIdentifier caps columns in one generated identifier.
	MaxDataColumnsPerIdentifier = 128
)

// DataColumnIdentifier is one caller-owned block-root and column list.
type DataColumnIdentifier struct {
	BlockRoot Root
	Columns   []uint64
}

// SSZDataColumnsByRootIdentifiers encodes exactly the caller-supplied identifiers.
func SSZDataColumnsByRootIdentifiers(identifiers []DataColumnIdentifier) ([]byte, error) {
	if len(identifiers) > MaxDataColumnRootIdentifiers {
		return nil, fmt.Errorf("data-column identifier count %d exceeds %d", len(identifiers), MaxDataColumnRootIdentifiers)
	}
	prefixSize := 4 * len(identifiers)
	totalSize := prefixSize
	for i, identifier := range identifiers {
		if len(identifier.Columns) > MaxDataColumnsPerIdentifier {
			return nil, fmt.Errorf("identifier %d has %d columns, exceeds %d", i, len(identifier.Columns), MaxDataColumnsPerIdentifier)
		}
		identifierSize := 36 + 8*len(identifier.Columns)
		if totalSize > math.MaxInt-identifierSize {
			return nil, fmt.Errorf("data-column identifier encoding length overflows int")
		}
		totalSize += identifierSize
	}
	out := make([]byte, totalSize)
	offset := prefixSize
	for i, identifier := range identifiers {
		binary.LittleEndian.PutUint32(out[i*4:], uint32(offset))
		copy(out[offset:], identifier.BlockRoot[:])
		binary.LittleEndian.PutUint32(out[offset+32:], 36)
		for j, column := range identifier.Columns {
			binary.LittleEndian.PutUint64(out[offset+36+j*8:], column)
		}
		offset += 36 + 8*len(identifier.Columns)
	}
	return out, nil
}
