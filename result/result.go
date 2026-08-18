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

// Verdict is a stable, machine-readable case classification:
//
//	ACCEPT      the target accepted the payload or kept the connection usable
//	DISCONNECT  the target disconnected because of the case
//	TIMEOUT     the target did not complete the required interaction in time
//	INVALID     the case is not applicable to the selected target or image
//	ERROR       Raidan could not execute or classify the case
//
// A case cannot report BUG or CRASH. The network worker assigns BUG from its
// approved log signatures. The target observer assigns CRASH from container
// death during the case window.
type Verdict string

const (
	// VerdictAccept means the target accepted the payload or kept the connection usable.
	VerdictAccept Verdict = "ACCEPT"
	// VerdictDisconnect means the target closed the connection because of the case.
	VerdictDisconnect Verdict = "DISCONNECT"
	// VerdictTimeout means the target did not complete the required interaction in time.
	VerdictTimeout Verdict = "TIMEOUT"
	// VerdictInvalid means the case is not applicable to the selected target or image.
	VerdictInvalid Verdict = "INVALID"
	// VerdictError means Raidan could not execute or classify the case.
	VerdictError Verdict = "ERROR"
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

// Invalid reports that the case is not applicable to the selected target or image.
func Invalid(detail string) Result {
	return Result{Verdict: VerdictInvalid, Detail: detail}
}

// Error reports that Raidan could not execute or classify the case.
func Error(detail string) Result {
	return Result{Verdict: VerdictError, Detail: detail}
}
