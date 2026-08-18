package sdk

import (
	"github.com/nethoxa-labs/raidan-sdk/result"
	"github.com/nethoxa-labs/raidan-sdk/session"
	"github.com/nethoxa-labs/raidan-sdk/target"
)

// Version identifies this SDK release.
const Version = "v0.2.2"

// Target describes the execution and consensus endpoints exercised by a case.
type Target = target.Target

// Result is the stable machine-readable outcome returned by every case.
type Result = result.Result

// Verdict classifies the outcome of a portable case.
type Verdict = result.Verdict

// Scope supplies case output, client identity, and write observation.
type Scope = session.Scope

// Observer receives protocol writes emitted by a case.
type Observer = session.Observer

// Write describes one protocol write observed during a case.
type Write = session.Write

// Test is the canonical portable case signature.
type Test = target.Test

const (
	// VerdictAccept indicates that the target accepted the payload or kept the connection usable.
	VerdictAccept = result.VerdictAccept
	// VerdictDisconnect indicates that the target disconnected because of the case.
	VerdictDisconnect = result.VerdictDisconnect
	// VerdictTimeout indicates that the target did not complete the required interaction in time.
	VerdictTimeout = result.VerdictTimeout
	// VerdictInvalid indicates that the case is not applicable to the selected target or image.
	VerdictInvalid = result.VerdictInvalid
	// VerdictError indicates that Raidan could not execute or classify the case.
	VerdictError = result.VerdictError
)
