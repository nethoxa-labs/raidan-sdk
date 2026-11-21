package snap

// Message codes are relative to the negotiated snap protocol offset.
const (
	GetAccountRangeCode     = 0x00
	AccountRangeCode        = 0x01
	GetStorageRangesCode    = 0x02
	StorageRangesCode       = 0x03
	GetByteCodesCode        = 0x04
	ByteCodesCode           = 0x05
	GetTrieNodesCode        = 0x06
	TrieNodesCode           = 0x07
	GetBlockAccessListsCode = 0x08
	BlockAccessListsCode    = 0x09
)
