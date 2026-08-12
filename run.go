package sdk

import (
	"context"
	"fmt"

	"github.com/nethoxa-labs/raidan-sdk/result"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// Run invokes test with output and client metadata attached to its context.
// Callers that need additional context values attach them before calling Run.
func Run(ctx context.Context, test Test, target Target, scope Scope) (outcome Result) {
	if test == nil {
		return result.Error("portable case function is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if scope.Client == "" && session.Client(ctx) == "" {
		scope.Client = target.Client
	}
	ctx = session.With(ctx, scope)
	if _, err := session.ParticipantIdentity(ctx); err != nil {
		return result.Error(err.Error())
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome = result.Error(fmt.Sprintf("portable case panicked: %v", recovered))
		}
	}()
	return test(ctx, target)
}
