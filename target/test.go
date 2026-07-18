package target

import (
	"context"

	"github.com/nethoxa-labs/raidan-sdk/result"
)

// Test is a portable workload callback that can be invoked by any Go program.
type Test func(context.Context, Target) result.Result
