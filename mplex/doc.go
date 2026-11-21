// Package mplex provides an explicit /mplex/6.7.0 transport for consensus
// peers.
//
// Dial opens one TCP connection to Target.ConsensusP2P. It runs
// multistream-select and Noise with the configured local identity. It then negotiates
// /mplex/6.7.0 and returns the connection to the caller.
//
// The caller owns the framing. Open, Data, Close, Reset, and Frame send one
// frame each, with a caller-chosen stream ID and payload. Drain is the only
// read path. The package starts no goroutine and reads nothing on its own.
//
// A connection has one identity and a cap on outbound bytes (MaxSend). Each
// frame is reported to the session observer with Protocol "mplex" and Raw set.
//
//	conn, err := mplex.Dial(ctx, target, 0)
//	if err != nil {
//		return err
//	}
//	defer conn.Shut()
//	if err := conn.Open(1, "stream"); err != nil {
//		return err
//	}
//	frames, err := conn.Drain(time.Second)
package mplex
