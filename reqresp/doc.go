// Package reqresp provides Ethereum consensus-layer request/response streams.
//
// It owns canonical protocol IDs, response codes, SSZ-snappy framing, beacon
// status discovery, libp2p sessions, and SSZ request encoders. Request is the
// one-shot API; NewSession reuses one libp2p host and peer connection for
// multiple streams.
//
//	response, err := reqresp.Request(
//		ctx,
//		target.ConsensusURL,
//		target.ConsensusP2P,
//		reqresp.PingV1,
//		reqresp.SSZUint64(0),
//		reqresp.RequestOptions{},
//	)
//	if err != nil {
//		return err
//	}
//	fmt.Println(response.Code)
//
// RawSSZSnappy, RawSSZSnappyWithVarint, WriteSSZSnappy, and ReadResponseChunk
// expose caller-controlled framing for applications that manage their own streams.
package reqresp
