package eth

// PooledTxPacket is the eth/66+ PooledTransactions response wrapper.
type PooledTxPacket struct {
	RequestID uint64
	Txs       RawTxList
}
