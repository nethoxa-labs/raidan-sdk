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

const (
	stepDim   = "\033[0;90m" // muted gray, so step narration stays in the background
	stepReset = "\033[0m"
)

// Step writes one muted per-operation narration line ("[+] …") to the scope
// output, but only when the scope is Verbose. Single-shot test cases enable it
// for a rich play-by-play; high-volume replay loops leave it off so
// their inner dials never flood the log.
func Step(ctx context.Context, format string, args ...any) {
	if !scopeFrom(ctx).Verbose {
		return
	}
	_, _ = fmt.Fprintf(Output(ctx), stepDim+format+stepReset+"\n", args...)
}
