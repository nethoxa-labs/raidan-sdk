// Package discovery provides Ethereum node discovery primitives.
//
// It exposes ENR encoding and signing, enode parsing, DNS discovery validation,
// discv4 packet types and UDP sessions, and discv5 handshake and post-session
// messages. DialDiscv4 and DialDiscv5 create reusable UDP sessions; packet
// encoders and session methods allow callers to control individual wire fields.
// All network entry points accept context.Context; reusable sessions retain it
// for cancellation, deadline capping, and outbound-write observation.
//
// A discv4 session can be opened directly from an enode URL:
//
//	conn, err := discovery.DialDiscv4(ctx, target.ExecutionP2P)
//	if err != nil {
//		return err
//	}
//	defer func() { _ = conn.Close() }()
//
// Use NewDiscv4PeerHarness when several exchanges should share one local peer
// identity. BuildSignedENR and the Discv5Conn APIs expose lower-level record and
// packet construction while leaving every field and identity caller-owned.
package discovery
