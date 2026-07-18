// Package rpc provides bounded execution-layer JSON-RPC access.
//
// Call performs a bounded JSON-RPC request and returns its raw result.
// FetchChainParams combines the genesis hash, head hash, state root, network
// ID, and fork ID required to establish an ETH peer session. Fork discovery
// requires admin_nodeInfo access on the execution endpoint.
//
//	params, err := rpc.FetchChainParams(ctx, target.ExecutionURL)
//	if err != nil {
//		return err
//	}
//	fmt.Println(params.NetworkID, params.HeadHash)
package rpc
