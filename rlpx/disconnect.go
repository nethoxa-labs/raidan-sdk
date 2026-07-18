package rlpx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
)

// DecodeDisconnectReason decodes a devp2p Disconnect payload. The clients
// in this repo emit the canonical list form, while several still accept the
// legacy scalar form, so we decode both and preserve large/unknown reasons.
func DecodeDisconnectReason(data []byte) (uint64, string) {
	if len(data) == 0 {
		return 0, "empty disconnect reason"
	}
	var reason uint64
	stream := rlp.NewStream(bytes.NewReader(data), 100)
	kind, _, err := stream.Kind()
	if err == nil && kind == rlp.List {
		_, _ = stream.List()
		err = stream.Decode(&reason)
	} else if err == nil {
		err = stream.Decode(&reason)
	}
	if err != nil {
		return 0xff, fmt.Sprintf("invalid disconnect reason payload: %v", err)
	}
	return reason, fmt.Sprintf("%s (%s)", disconnectReasonName(reason), disconnectReasonHex(reason))
}

func disconnectReasonName(reason uint64) string {
	switch reason {
	case 0x00:
		return "disconnect requested"
	case 0x01:
		return "network error"
	case 0x02:
		return "breach of protocol"
	case 0x03:
		return "useless peer"
	case 0x04:
		return "too many peers"
	case 0x05:
		return "already connected"
	case 0x06:
		return "incompatible p2p protocol version"
	case 0x07:
		return "invalid node identity"
	case 0x08:
		return "client quitting"
	case 0x09:
		return "unexpected identity"
	case 0x0a:
		return "connected to self"
	case 0x0b:
		return "read timeout"
	case 0x10:
		return "subprotocol error"
	case 0xff:
		return "invalid disconnect reason"
	default:
		return fmt.Sprintf("unknown disconnect reason %d", reason)
	}
}

func disconnectReasonHex(reason uint64) string {
	if reason <= 0xff {
		return fmt.Sprintf("0x%02x", reason)
	}
	return fmt.Sprintf("0x%x", reason)
}

// DisconnectDetail formats an optional decoded disconnect reason.
func DisconnectDetail(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "peer disconnected"
	}
	return "peer disconnected: " + reason
}
