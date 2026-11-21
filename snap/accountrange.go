package snap

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// AccountData is one entry in an AccountRange response.
type AccountData struct {
	Hash common.Hash
	Body rlp.RawValue
}

// AccountRangePacket is the snap/1 AccountRange response.
type AccountRangePacket struct {
	ID       uint64
	Accounts []AccountData
	Proof    [][]byte
}

// SetRequestID sets the response correlation identifier.
func (p *AccountRangePacket) SetRequestID(id uint64) { p.ID = id }
