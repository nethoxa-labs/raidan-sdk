package session

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"io"
	"os"

	"github.com/nethoxa-labs/raidan-sdk/utils"
)

// Scope carries optional operation metadata through a context.
type Scope struct {
	Output     io.Writer
	Client     string
	Observer   Observer
	Identities []*utils.ParticipantIdentity
	// Verbose enables per-step narration (Step). It is set for single-shot test
	// cases and left off for high-volume replay loops that would flood.
	Verbose bool
	// Quiet explicitly suppresses inherited verbose narration for child scopes.
	Quiet bool
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
	if len(scope.Identities) == 0 {
		scope.Identities = parent.Identities
	}
	if scope.Quiet {
		scope.Verbose = false
	} else if !scope.Verbose {
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

// ParticipantIdentity returns the required case-scoped protocol identity.
func ParticipantIdentity(ctx context.Context) (*utils.ParticipantIdentity, error) {
	return ParticipantIdentityAt(ctx, 0)
}

// ParticipantIdentityAt returns one exact peer identity for the current case.
func ParticipantIdentityAt(ctx context.Context, index int) (*utils.ParticipantIdentity, error) {
	identities := scopeFrom(ctx).Identities
	if index < 0 || index >= len(identities) || identities[index] == nil {
		return nil, errors.New("participant identity is not configured")
	}
	return identities[index], nil
}

// ParticipantELKey returns the execution-layer key for the current case.
func ParticipantELKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	return ParticipantELKeyAt(ctx, 0)
}

// ParticipantELKeyAt returns one execution-layer key for the current case.
func ParticipantELKeyAt(ctx context.Context, index int) (*ecdsa.PrivateKey, error) {
	identity, err := ParticipantIdentityAt(ctx, index)
	if err != nil {
		return nil, err
	}
	return identity.ELKey()
}

func scopeFrom(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	scope, _ := ctx.Value(scopeKey{}).(Scope)
	return scope
}
