// Package target describes the HTTP and peer-to-peer endpoints of an Ethereum
// node pair.
//
// Discover validates a Target, resolves a missing execution enode through
// admin_nodeInfo and, when a consensus API is configured, resolves its libp2p
// address or ENR through /eth/v1/node/identity.
//
//	node, err := target.Discover(
//		ctx,
//		target.Target{
//			ExecutionURL: "http://127.0.0.1:8545",
//			ConsensusURL: "http://127.0.0.1:5052",
//		},
//	)
package target
