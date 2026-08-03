package oidc

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
)

// Reading claims out of a verified ID token.
//
// It is more forgiving than the rest of this package on purpose. Verification is
// where a provider is held to the specification exactly, because that is where
// being lax is a security property being given away. This is where five
// providers that all disagree about how to spell a display name are turned into
// one struct, and being strict here buys nothing but an integration that does
// not work.
//
// What is *not* forgiving: `sub` is required, and `email_verified` is false
// unless the provider actually said true. Everything decided from those two is
// decided in internal/authn.

// nameClaims are the claims a display name might be in, best first. `name` is
// the standard one; the rest are what providers send when a directory has no
// formatted name — Keycloak and Okta send `preferred_username`, Entra sends
// `upn` for a federated account.
var nameClaims = []string{"name", "preferred_username", "given_name", "upn"}

// identityOf turns a verified token into the claims this deployment acts on.
func (p *Provider) identityOf(token *coreoidc.IDToken) (Identity, error) {
	var claims map[string]json.RawMessage
	if err := token.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("%w: the ID token's claims could not be read: %w", ErrRejected, err)
	}

	if strings.TrimSpace(token.Subject) == "" {
		// Unreachable through a conforming provider — `sub` is required by the
		// specification — and worth refusing out loud rather than linking an
		// account to the empty string.
		return Identity{}, fmt.Errorf("%w: the ID token carries no subject", ErrRejected)
	}

	identity := Identity{
		Subject:       token.Subject,
		Email:         strings.TrimSpace(stringClaim(claims, "email")),
		EmailVerified: boolClaim(claims, "email_verified"),
		Groups:        stringsClaim(claims, p.groupsClaim),
	}
	for _, claim := range nameClaims {
		if name := strings.TrimSpace(stringClaim(claims, claim)); name != "" {
			identity.DisplayName = name
			break
		}
	}
	if identity.DisplayName == "" {
		// Something has to be shown next to their comments. The address is what
		// the account is known by anyway, and an empty name would be a blank in
		// every interface that renders one.
		identity.DisplayName = identity.Email
	}
	return identity, nil
}

// stringClaim reads a claim that should be a string, and returns "" for one that
// is absent or is not.
func stringClaim(claims map[string]json.RawMessage, name string) string {
	raw, ok := claims[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// boolClaim reads a claim that should be a boolean, and tolerates the string
// spelling of one.
//
// The tolerance is for a real provider: several send `"email_verified": "true"`,
// because the value came out of a directory as text and was never converted.
// Reading that as false would mean refusing to link an address the provider
// considers verified — the safe direction, but wrong often enough that it looks
// like a bug in this application rather than in theirs.
//
// Anything else, including a missing claim, is false. That is the direction that
// costs an account nothing: an unverified address links nothing and provisions
// under its own subject.
func boolClaim(claims map[string]json.RawMessage, name string) bool {
	raw, ok := claims[name]
	if !ok {
		return false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return false
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(text))
	return err == nil && parsed
}

// stringsClaim reads a claim that should be a list of strings, and accepts the
// three shapes providers send: a list, a single string, and a space-separated
// string.
//
// A single string is what a directory with one group per user produces; the
// space-separated form is how some providers spell a multi-valued attribute.
// Values that are not strings are skipped rather than failing the whole claim: a
// group list with a number in it should not stop somebody signing in, it should
// map no role.
func stringsClaim(claims map[string]json.RawMessage, name string) []string {
	raw, ok := claims[name]
	if !ok {
		return nil
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		values := make([]string, 0, len(list))
		for _, item := range list {
			var value string
			if err := json.Unmarshal(item, &value); err == nil && strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
		return values
	}

	var single string
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil
	}
	return strings.Fields(single)
}

// constantTimeEqual compares two values without leaking how much of them
// matched. It is here for the nonce, and is the same comparison the state gets.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
