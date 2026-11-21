package snap

import (
	"fmt"
	"time"

	ethsdk "github.com/nethoxa-labs/raidan-sdk/eth"
)

const solicitedResponseWait = 5 * time.Second

// WaitForRequestID waits for a SNAP request on an established ETH/SNAP
// connection and returns its request ID and encoded payload.
func WaitForRequestID(conn *ethsdk.Conn, requestCode uint64, label string, timeout time.Duration) (uint64, []byte, error) {
	if timeout <= 0 {
		timeout = solicitedResponseWait
	}
	data, err := conn.WaitForMsg(timeout, conn.SNAPOffset()+requestCode)
	if err != nil {
		return 0, nil, fmt.Errorf("target did not send %s within %s: %w", label, timeout, err)
	}
	id, err := ethsdk.DecodeRequestID(data)
	if err != nil {
		return 0, nil, fmt.Errorf("decode %s request id: %w", label, err)
	}
	return id, data, nil
}

// RequestForResponse returns the request code paired with a SNAP response.
func RequestForResponse(responseCode uint64) (uint64, string, error) {
	switch responseCode {
	case AccountRangeCode:
		return GetAccountRangeCode, "GetAccountRange", nil
	case StorageRangesCode:
		return GetStorageRangesCode, "GetStorageRanges", nil
	case ByteCodesCode:
		return GetByteCodesCode, "GetByteCodes", nil
	case TrieNodesCode:
		return GetTrieNodesCode, "GetTrieNodes", nil
	case BlockAccessListsCode:
		return GetBlockAccessListsCode, "GetBlockAccessLists", nil
	default:
		return 0, "", fmt.Errorf("snap code 0x%x is not a known response code", responseCode)
	}
}
