package result

import (
	"context"
	"fmt"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const (
	red    = "\033[0;31m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	gray   = "\033[0;90m"
	reset  = "\033[0m"
)

// Progress writes a gray diagnostic line to the active session output.
func Progress(ctx context.Context, format string, args ...any) {
	_, _ = fmt.Fprintf(session.Output(ctx), "%s"+format+"%s\n",
		append([]any{gray}, append(args, reset)...)...)
}

// Result classifies an operation and carries its human-readable detail.
type Result struct {
	Verdict Verdict
	Detail  string
}

// Verdict is a stable, machine-readable operation classification:
//
//	ACCEPT      the connection was still alive
//	DISCONNECT  the target disconnected us
//	TIMEOUT     the target did not respond within the wait window
//	INVALID     the operation could not reach a meaningful verdict
//	BUG         a product-level bug was detected
//	CRASH       the target client crashed
type Verdict string

const (
	// VerdictAccept means the target behaved as expected.
	VerdictAccept Verdict = "ACCEPT"
	// VerdictDisconnect means the target unexpectedly closed the connection.
	VerdictDisconnect Verdict = "DISCONNECT"
	// VerdictTimeout means the target did not respond within the allowed window.
	VerdictTimeout Verdict = "TIMEOUT"
	// VerdictInvalid means the case could not reach a meaningful conclusion.
	VerdictInvalid Verdict = "INVALID"
	// VerdictBug means the target violated the case expectation.
	VerdictBug Verdict = "BUG"
	// VerdictCrash means the target process terminated during the case.
	VerdictCrash Verdict = "CRASH"
)

// Print writes one colored, human-readable verdict line. Keeping rendering
// separate from construction makes Result safe to create in libraries and
// straightforward to report once at an application boundary.
func Print(ctx context.Context, outcome Result) {
	color := yellow
	switch outcome.Verdict {
	case VerdictAccept:
		color = green
	case VerdictDisconnect:
		color = red
	}
	if outcome.Detail == "" {
		_, _ = fmt.Fprintf(session.Output(ctx), "%s%s%s\n", color, outcome.Verdict, reset)
		return
	}
	_, _ = fmt.Fprintf(session.Output(ctx), "%s%s%s — %s\n", color, outcome.Verdict, reset, outcome.Detail)
}

// Accept reports an accepted operation.
func Accept(detail string) Result {
	return Result{Verdict: VerdictAccept, Detail: detail}
}

// Disconnect reports that the remote peer closed the connection.
func Disconnect(detail string) Result {
	return Result{Verdict: VerdictDisconnect, Detail: detail}
}

// Timeout reports that an operation exceeded its response window.
func Timeout(detail string) Result {
	return Result{Verdict: VerdictTimeout, Detail: detail}
}

// Invalid reports that an error prevented a meaningful classification.
func Invalid(detail string) Result {
	return Result{Verdict: VerdictInvalid, Detail: detail}
}

// Bug reports protocol behavior that violates the case expectation.
func Bug(detail string) Result {
	return Result{Verdict: VerdictBug, Detail: detail}
}

// Crash reports that the target process terminated while handling the case.
func Crash(detail string) Result {
	return Result{Verdict: VerdictCrash, Detail: detail}
}
