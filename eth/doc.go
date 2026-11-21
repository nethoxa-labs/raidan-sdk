// Package eth provides the execution-layer ETH devp2p capability.
//
// It exposes ETH message codes and packet types, Status negotiation, request ID
// matching, raw and structured message sending, and canonical ETH packet
// shapes. Dial creates a ready session; Config selects the maximum ETH version,
// optional SNAP version, or an exact capability list. DialPreStatus stops after
// devp2p Hello for callers that need to craft the initial Status exchange.
//
//	conn, err := eth.Dial(ctx, target.ExecutionP2P, target.ExecutionURL, eth.Config{})
//	if err != nil {
//		return err
//	}
//	defer conn.Close()
//	fmt.Printf("negotiated eth/%d\n", conn.ETHVersion())
//
// Conn retains ctx for cancellation, deadline capping, and write observation.
// It supports structured sends, exact pre-encoded RLP payloads, bounded reads,
// liveness checks, request matching, disconnect reasons, and negotiated offsets.
// Message structs live in files named for their wire message; their exported
// field order is their RLP field order.
package eth
