package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// WireLegacyTx is a caller-owned legacy transaction wire shape.
type WireLegacyTx struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *common.Address
	Value    *big.Int
	Data     []byte
	V        *big.Int
	R        *big.Int
	S        *big.Int
}

// WireHeader is a caller-owned execution header wire shape.
type WireHeader struct {
	ParentHash      common.Hash
	UncleHash       common.Hash
	Coinbase        common.Address
	Root            common.Hash
	TxHash          common.Hash
	ReceiptHash     common.Hash
	Bloom           [256]byte
	Difficulty      *big.Int
	Number          *big.Int
	GasLimit        uint64
	GasUsed         uint64
	Time            uint64
	Extra           []byte
	MixDigest       common.Hash
	Nonce           [8]byte
	BaseFee         *big.Int
	WithdrawalsHash *common.Hash `rlp:"optional"`
}

// WireBlockBody is a caller-owned legacy execution block body.
type WireBlockBody struct {
	Txs    []rlp.RawValue
	Uncles []*WireHeader
}

// WireBlockBodyShanghai is a caller-owned withdrawals-aware block body.
type WireBlockBodyShanghai struct {
	Txs         []rlp.RawValue
	Uncles      []*WireHeader
	Withdrawals []*WireWithdrawal
}

// WireWithdrawal is a caller-owned execution-layer withdrawal.
type WireWithdrawal struct {
	Index          uint64
	ValidatorIndex uint64
	Address        common.Address
	Amount         uint64
}

// WireBlock is a caller-owned execution block.
type WireBlock struct {
	Header *WireHeader
	Txs    []rlp.RawValue
	Uncles []*WireHeader
}

// WireNewBlockData is a caller-owned ETH NewBlock payload.
type WireNewBlockData struct {
	Block WireBlock
	TD    *big.Int
}

// WireLog is a caller-owned execution receipt log.
type WireLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

// WireReceipt is a caller-owned execution receipt.
type WireReceipt struct {
	Status            uint64
	CumulativeGasUsed uint64
	Bloom             [256]byte
	Logs              []*WireLog
}

// WireBlockHeadersPacket is a caller-owned BlockHeaders response.
type WireBlockHeadersPacket struct {
	RequestID uint64
	Headers   []*WireHeader
}

// WireBlockBodiesPacket is a caller-owned BlockBodies response.
type WireBlockBodiesPacket struct {
	RequestID uint64
	Bodies    []*WireBlockBody
}

// WireReceiptsPacket is a caller-owned Receipts response.
type WireReceiptsPacket struct {
	RequestID uint64
	Receipts  [][]*WireReceipt
}
