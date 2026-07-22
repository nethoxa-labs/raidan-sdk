package session

import (
	"context"
	"io"
	"os"
)

// Scope carries optional operation metadata through a context.
type Scope struct {
	Output   io.Writer
	Client   string
	Observer Observer
	// Verbose enables per-step narration (Step). It is set for single-shot test
	// cases and left off for high-volume replay loops that would flood.
	Verbose bool
}

type scopeKey struct{}

// With returns a context containing scope. Empty fields inherit their value
// from the parent context, which makes nested protocol operations cheap and
// predictable.
func With(ctx context.Context, scope Scope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	parent := scopeFrom(ctx)
	if scope.Output == nil {
		scope.Output = parent.Output
	}
	if scope.Client == "" {
		scope.Client = parent.Client
	}
	if scope.Observer == nil {
		scope.Observer = parent.Observer
	}
	if !scope.Verbose {
		scope.Verbose = parent.Verbose
	}
	return context.WithValue(ctx, scopeKey{}, scope)
}

// Output returns the writer attached to ctx, or stdout when none is attached.
func Output(ctx context.Context) io.Writer {
	if output := scopeFrom(ctx).Output; output != nil {
		return output
	}
	return os.Stdout
}

// Client returns the client label attached to ctx.
func Client(ctx context.Context) string { return scopeFrom(ctx).Client }

func scopeFrom(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	scope, _ := ctx.Value(scopeKey{}).(Scope)
	return scope
}
