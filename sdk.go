package sdk

import (
	"github.com/nethoxa-labs/raidan-sdk/result"
	"github.com/nethoxa-labs/raidan-sdk/session"
	"github.com/nethoxa-labs/raidan-sdk/target"
)

// Version identifies this SDK release.
const Version = "v0.1.2"

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
	// VerdictAccept indicates that the target behaved as expected.
	VerdictAccept = result.VerdictAccept
	// VerdictDisconnect indicates that the target disconnected unexpectedly.
	VerdictDisconnect = result.VerdictDisconnect
	// VerdictTimeout indicates that the target did not respond in time.
	VerdictTimeout = result.VerdictTimeout
	// VerdictInvalid indicates an invalid or inconclusive target response.
	VerdictInvalid = result.VerdictInvalid
	// VerdictBug indicates a protocol bug in the target.
	VerdictBug = result.VerdictBug
	// VerdictCrash indicates that the target crashed.
	VerdictCrash = result.VerdictCrash
)
