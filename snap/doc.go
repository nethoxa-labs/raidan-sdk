// Package snap provides the SNAP state-sync capability and its packet model.
//
// It exposes SNAP message codes and RLP packet types for account ranges,
// storage ranges, bytecodes, trie nodes, and block access lists.
// RequestForResponse and WaitForRequestID support solicited exchanges over an
// eth.Conn negotiated with eth.Config{SnapVersion: 1} or SnapVersion: 2.
// eth.Conn.SendSnap and SendSnapRaw send structured or exact pre-encoded
// payloads. Response packet types expose SetRequestID for typed correlation.
//
//	requestCode, name, err := snap.RequestForResponse(snap.AccountRangeCode)
//	if err != nil {
//		return err
//	}
//	fmt.Printf("%s uses request code %#x\n", name, requestCode)
package snap
