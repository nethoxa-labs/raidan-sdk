package gossipsub

import "encoding/binary"

// ExecutionProofTopic is the EIP-8025 global gossip topic name.
const ExecutionProofTopic = "execution_proof"

// SignedExecutionProof contains the caller-owned EIP-8025 proof fields.
type SignedExecutionProof struct {
	ProofData             []byte
	ProofType             byte
	NewPayloadRequestRoot [32]byte
	ValidatorIndex        uint64
	Signature             [96]byte
}

// SSZSignedExecutionProof encodes one EIP-8025 SignedExecutionProof.
func SSZSignedExecutionProof(proof SignedExecutionProof) []byte {
	const signedFixedSize = 108
	const proofFixedSize = 37
	out := make([]byte, signedFixedSize+proofFixedSize+len(proof.ProofData))
	binary.LittleEndian.PutUint32(out[0:4], signedFixedSize)
	binary.LittleEndian.PutUint64(out[4:12], proof.ValidatorIndex)
	copy(out[12:108], proof.Signature[:])
	binary.LittleEndian.PutUint32(out[108:112], proofFixedSize)
	out[112] = proof.ProofType
	copy(out[113:145], proof.NewPayloadRequestRoot[:])
	copy(out[145:], proof.ProofData)
	return out
}
