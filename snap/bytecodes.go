package snap

// ByteCodesPacket is the snap/1 ByteCodes response.
type ByteCodesPacket struct {
	ID    uint64
	Codes [][]byte
}

// SetRequestID sets the response correlation identifier.
func (p *ByteCodesPacket) SetRequestID(id uint64) { p.ID = id }
