package snap

// TrieNodesPacket is the snap/1 TrieNodes response.
type TrieNodesPacket struct {
	ID    uint64
	Nodes [][]byte
}

// SetRequestID sets the response correlation identifier.
func (p *TrieNodesPacket) SetRequestID(id uint64) { p.ID = id }
