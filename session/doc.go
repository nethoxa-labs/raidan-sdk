// Package session carries output, client metadata, and write observation in a
// standard context.Context.
//
// Attach a scope at an application boundary and pass the returned context to
// protocol operations:
//
//	ctx = session.With(ctx, session.Scope{Output: logWriter, Client: "geth"})
//
// Remaining gives protocol operations the case deadline when one exists. Printf and
// Println write to Scope.Output. Scope.Observer receives defensive copies of
// outbound protocol writes for tracing or metrics.
//
// # Delivery events
//
// When the environment variable RAIDAN_EVENTS names a file, the connection
// layers (rlpx, eth and snap over devp2p, libp2p req/resp and gossipsub, discv4
// and discv5) append one JSON object per line to it, recording what the program
// did to the peer and what the peer did back. With the variable unset nothing
// is written. Every line has "time" (UTC, RFC 3339) and "event", and the
// fields that apply:
//
//	connected     protocol, peer, addr (eth: kind lists the negotiated capabilities)
//	sent          protocol, peer, addr, kind, size
//	reply         protocol, peer, addr, kind, size, status (req/resp response code)
//	disconnected  protocol, peer, addr, reason_code, reason (the peer's Disconnect
//	              or Goodbye reason, including a breach of protocol; a closed
//	              connection without a reason has only reason text)
//	error         protocol, peer, op, error (a failed dial, handshake, write, read or request)
//
// kind is the message code relative to its capability for eth and snap (for
// example "0x03" with protocol "eth"), the protocol ID for req/resp, the topic
// for gossipsub, and the packet type for discovery. Events are appended
// through one locked write per line, so concurrent connections share a file.
// A loop that sends many messages writes a line for each.
package session
