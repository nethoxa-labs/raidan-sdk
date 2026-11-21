// Package rlpx provides Ethereum's encrypted RLPx transport and devp2p base
// protocol primitives.
//
// DialRLPx performs the encrypted authentication exchange. DialAndHello
// additionally writes an exact caller-supplied Hello. The
// package also exposes Hello wire types, disconnect decoding, raw frame writes,
// and bounded close detection.
//
//	conn, network, err := rlpx.DialRLPx(ctx, target.ExecutionP2P)
//	if err != nil {
//		return err
//	}
//	defer network.Close()
//	_ = conn
//
// The eth, snap, and wit packages build their higher-level sessions on these
// transport helpers.
package rlpx
