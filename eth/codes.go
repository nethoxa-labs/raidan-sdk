package eth

// ETH message codes (relative to the eth protocol offset).
const (
	EthStatus                     = 0x00
	EthNewBlockHashes             = 0x01
	EthTransactions               = 0x02
	EthGetBlockHeaders            = 0x03
	EthBlockHeaders               = 0x04
	EthGetBlockBodies             = 0x05
	EthBlockBodies                = 0x06
	EthNewBlock                   = 0x07
	EthNewPooledTransactionHashes = 0x08
	EthGetPooledTransactions      = 0x09
	EthPooledTransactions         = 0x0A
	EthGetReceipts                = 0x0F
	EthReceipts                   = 0x10
	EthBlockRangeUpdate           = 0x11
	EthGetBlockAccessLists        = 0x12
	EthBlockAccessLists           = 0x13
	EthGetCells                   = 0x14
	EthCells                      = 0x15
)
