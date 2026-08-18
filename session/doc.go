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
package session
