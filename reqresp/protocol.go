package reqresp

import (
	"fmt"
	"strings"
)

const (
	// ProtocolPrefix is the consensus request/response namespace.
	ProtocolPrefix = "/eth2/beacon_chain/req"
	// Encoding is the consensus request/response encoding component.
	Encoding = "ssz_snappy"

	// CodeSuccess identifies a successful response chunk.
	CodeSuccess byte = 0
	// CodeInvalidRequest identifies the protocol-defined invalid-request response.
	CodeInvalidRequest byte = 1
	// CodeServerError identifies a responder-side failure.
	CodeServerError byte = 2
	// CodeResourceUnavailable identifies data unavailable at the responder.
	CodeResourceUnavailable byte = 3

	// ContextBytesLen is the fork-digest context length on contextual responses.
	ContextBytesLen = 4
	// MaxPayloadSize is the hard decoded ceiling for one response chunk.
	MaxPayloadSize = 10 * 1024 * 1024
	// MaxTotalResponseSize is the aggregate decoded-byte ceiling per request.
	MaxTotalResponseSize = 64 * 1024 * 1024
	// MaxResponseChunks is the response-frame ceiling per request.
	MaxResponseChunks uint64 = 1024
)

// ProtocolID returns the canonical request/response protocol identifier.
func ProtocolID(message string, version int) (string, error) {
	if version < 0 || version > 999 {
		return "", fmt.Errorf("protocol version %d is outside 0..999", version)
	}
	message = strings.Trim(message, "/")
	if message == "" {
		return "", fmt.Errorf("protocol message is empty")
	}
	return fmt.Sprintf("%s/%s/%d/%s", ProtocolPrefix, message, version, Encoding), nil
}
