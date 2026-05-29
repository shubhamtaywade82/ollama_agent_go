// Package governance provides authentication, authorization, PII scrubbing,
// secret scanning, and prompt guardrails for the agent runtime.
package governance

import (
	"context"
	"fmt"
)

// Identity represents an authenticated principal.
type Identity struct {
	UserID string
	OrgID  string
	Roles  []string
	Token  string
}

// AuthProvider verifies bearer tokens and returns the associated Identity.
type AuthProvider interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

// StaticTokenAuth maps static bearer tokens to identities (suitable for local / single-user).
type StaticTokenAuth struct {
	tokens map[string]Identity
}

// NewStaticTokenAuth creates an auth provider backed by a static token map.
func NewStaticTokenAuth(tokens map[string]Identity) *StaticTokenAuth {
	return &StaticTokenAuth{tokens: tokens}
}

// Verify looks up the token and returns the associated Identity.
func (a *StaticTokenAuth) Verify(_ context.Context, token string) (Identity, error) {
	if id, ok := a.tokens[token]; ok {
		return id, nil
	}
	return Identity{}, fmt.Errorf("governance: invalid or unknown token")
}

// AnonymousIdentity is used when no auth is configured.
var AnonymousIdentity = Identity{UserID: "anonymous", OrgID: "", Roles: []string{"operator"}}

// NoAuth always returns AnonymousIdentity (used when auth is disabled).
type NoAuth struct{}

func (NoAuth) Verify(_ context.Context, _ string) (Identity, error) {
	return AnonymousIdentity, nil
}
