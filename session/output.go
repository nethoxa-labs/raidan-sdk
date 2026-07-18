package session

import (
	"context"
	"fmt"
)

// Printf writes formatted output to the scope carried by ctx.
func Printf(ctx context.Context, format string, args ...any) {
	_, _ = fmt.Fprintf(Output(ctx), format, args...)
}

// Println writes a line to the scope carried by ctx.
func Println(ctx context.Context, args ...any) {
	_, _ = fmt.Fprintln(Output(ctx), args...)
}
