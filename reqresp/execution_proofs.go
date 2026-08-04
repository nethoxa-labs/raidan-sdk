package reqresp

import (
	"encoding/binary"
	"fmt"
)

const (
	// ExecutionProofsByRangeV1 is the EIP-8025 range request protocol ID.
	ExecutionProofsByRangeV1 = ProtocolPrefix + "/execution_proofs_by_range/1/" + Encoding
	// ExecutionProofsByRootV1 is the EIP-8025 root request protocol ID.
	ExecutionProofsByRootV1 = ProtocolPrefix + "/execution_proofs_by_root/1/" + Encoding
	// ExecutionProofStatusV1 is the EIP-8025 proof status protocol ID.
	ExecutionProofStatusV1 = ProtocolPrefix + "/execution_proof_status/1/" + Encoding

	// MaxExecutionProofTypes is the EIP-8025 proof type list bound.
	MaxExecutionProofTypes = 4
	// MaxExecutionProofRootIdentifiers is the Deneb request block bound.
	MaxExecutionProofRootIdentifiers = 128
)

// ProofByRootIdentifier selects proof types for one block root.
type ProofByRootIdentifier struct {
	BlockRoot  Root
	ProofTypes []byte
}

// SSZExecutionProofsByRange encodes the EIP-8025 range request container.
func SSZExecutionProofsByRange(startSlot, count uint64, proofTypes []byte) ([]byte, error) {
	if len(proofTypes) > MaxExecutionProofTypes {
		return nil, fmt.Errorf("proof type count %d exceeds %d", len(proofTypes), MaxExecutionProofTypes)
	}
	out := make([]byte, 20+len(proofTypes))
	binary.LittleEndian.PutUint64(out[0:8], startSlot)
	binary.LittleEndian.PutUint64(out[8:16], count)
	binary.LittleEndian.PutUint32(out[16:20], 20)
	copy(out[20:], proofTypes)
	return out, nil
}

// SSZExecutionProofsByRoot encodes the EIP-8025 variable identifier list.
func SSZExecutionProofsByRoot(identifiers []ProofByRootIdentifier) ([]byte, error) {
	if len(identifiers) > MaxExecutionProofRootIdentifiers {
		return nil, fmt.Errorf("proof identifier count %d exceeds %d", len(identifiers), MaxExecutionProofRootIdentifiers)
	}
	total := 4 * len(identifiers)
	for index, identifier := range identifiers {
		if len(identifier.ProofTypes) > MaxExecutionProofTypes {
			return nil, fmt.Errorf("identifier %d proof type count %d exceeds %d", index, len(identifier.ProofTypes), MaxExecutionProofTypes)
		}
		total += 36 + len(identifier.ProofTypes)
	}
	out := make([]byte, total)
	offset := 4 * len(identifiers)
	for index, identifier := range identifiers {
		binary.LittleEndian.PutUint32(out[index*4:index*4+4], uint32(offset))
		copy(out[offset:offset+32], identifier.BlockRoot[:])
		binary.LittleEndian.PutUint32(out[offset+32:offset+36], 36)
		copy(out[offset+36:], identifier.ProofTypes)
		offset += 36 + len(identifier.ProofTypes)
	}
	return out, nil
}

// SSZExecutionProofStatus encodes the EIP-8025 status request and response.
func SSZExecutionProofStatus(blockRoot Root, slot uint64, proofTypes []byte) ([]byte, error) {
	if len(proofTypes) > MaxExecutionProofTypes {
		return nil, fmt.Errorf("proof type count %d exceeds %d", len(proofTypes), MaxExecutionProofTypes)
	}
	out := make([]byte, 44+len(proofTypes))
	copy(out[0:32], blockRoot[:])
	binary.LittleEndian.PutUint64(out[32:40], slot)
	binary.LittleEndian.PutUint32(out[40:44], 44)
	copy(out[44:], proofTypes)
	return out, nil
}
