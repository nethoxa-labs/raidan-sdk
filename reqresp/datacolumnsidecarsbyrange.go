package reqresp

import (
	"encoding/binary"
	"fmt"
)

// DataColumnsByRangeV1 is the DataColumnSidecarsByRange v1 protocol ID.
const DataColumnsByRangeV1 = ProtocolPrefix + "/data_column_sidecars_by_range/1/" + Encoding

// MaxDataColumnsByRange is the SDK allocation ceiling for encoded columns.
const MaxDataColumnsByRange = 1 << 20

// SSZDataColumnsByRangeRequest encodes a DataColumnSidecarsByRange request.
func SSZDataColumnsByRangeRequest(startSlot, count uint64, columns []uint64) ([]byte, error) {
	if len(columns) > MaxDataColumnsByRange {
		return nil, fmt.Errorf("data column count %d exceeds %d", len(columns), MaxDataColumnsByRange)
	}
	out := make([]byte, 20+8*len(columns))
	binary.LittleEndian.PutUint64(out[0:8], startSlot)
	binary.LittleEndian.PutUint64(out[8:16], count)
	binary.LittleEndian.PutUint32(out[16:20], 20)
	for i, column := range columns {
		binary.LittleEndian.PutUint64(out[20+i*8:], column)
	}
	return out, nil
}
