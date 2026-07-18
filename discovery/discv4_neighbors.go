package discovery

import (
	"net"

	"github.com/ethereum/go-ethereum/rlp"
)

// Discv4NeighborsType is the discv4 Neighbors packet type.
const Discv4NeighborsType = 0x04

// Discv4Neighbors is the discv4 Neighbors packet body.
type Discv4Neighbors struct {
	Nodes      []Discv4NodeEntry
	Expiration uint64
	Rest       []rlp.RawValue `rlp:"tail"`
}

// Discv4NodeEntry is one endpoint advertised by Neighbors.
type Discv4NodeEntry struct {
	IP  net.IP
	UDP uint16
	TCP uint16
	ID  [64]byte
}
