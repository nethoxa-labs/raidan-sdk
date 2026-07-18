package sdk

import (
	"context"

	"github.com/nethoxa-labs/raidan-sdk/result"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// Run invokes test with output and client metadata attached to its context.
// Callers that need additional context values attach them before calling Run.
func Run(ctx context.Context, test Test, target Target, scope Scope) Result {
	if test == nil {
		return result.Invalid("portable case function is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if scope.Client == "" && session.Client(ctx) == "" {
		scope.Client = target.Client
	}
	ctx = session.With(ctx, scope)
	return test(ctx, target)
}
