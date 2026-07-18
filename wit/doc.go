// Package wit implements the WIT/1 execution-witness devp2p capability.
// Dial negotiates RLPx, Hello, ETH Status, and WIT/1; Conn then writes
// pre-encoded WIT messages and checks peer liveness.
//
//	conn, err := wit.Dial(ctx, target.ExecutionP2P, target.ExecutionURL)
//	if err != nil {
//		return err
//	}
//	defer conn.Close()
//	pages := []wit.PageRequest{{}}
//	payload, err := wit.EncodeGetWitness(1, pages)
//	if err != nil {
//		return err
//	}
//	return conn.SendRaw(wit.GetWitnessMsg, payload)
//
// Each wire message has its code, types, and encoder in its namesake source
// file. ErrUnsupported reports a peer that did not negotiate WIT/1.
package wit
