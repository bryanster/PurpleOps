package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"golang.org/x/sync/singleflight"
)

// The key set behind ID token verification, and the one part of the protocol
// this package implements rather than delegating.
//
// go-oidc ships a perfectly good remote key set, and it is not used here for one
// reason: it refetches the JWKS every time it meets a key ID it does not
// recognise, with nothing in front of that. A token signed with a key that does
// not exist is free to produce, and the callback endpoint is reachable by
// anybody — so a few hundred of them a second turn this server into a load
// generator aimed at the identity provider, and the identity provider is what
// every other application in the estate signs in through. M1-009 asks for the
// refetch (a rotation must be handled without a restart) *and* for a limit on
// it, which is what this file adds.
//
// Everything else about it is deliberately ordinary: keys are matched by `kid`,
// the algorithm is checked against an allowlist before a signature is verified,
// and the whole set is replaced on each fetch rather than merged — a key the
// provider has withdrawn must stop verifying tokens.

// signingAlgorithms are the algorithms an ID token may be signed with: the
// asymmetric ones, and nothing else.
//
// The list is an allowlist rather than "whatever the token says" because the two
// classic JWT attacks are both algorithm confusion — `alg: none`, and an HMAC
// signed with the provider's *public* key, which a verifier that trusts the
// header will happily check. Neither is expressible against this list.
var signingAlgorithms = []jose.SignatureAlgorithm{
	jose.RS256, jose.RS384, jose.RS512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.ES256, jose.ES384, jose.ES512,
}

// maxJWKSBytes caps what a JWKS response may be. A provider's key set is a few
// kilobytes; this is three orders of magnitude of headroom, and it means a
// misconfigured URL pointing at something enormous is a failed verification
// rather than a memory problem.
const maxJWKSBytes = 1 << 20

// defaultRefetchInterval is the minimum gap between two fetches of the same key
// set. It is the whole rate limit, and the number is a trade-off in both
// directions.
//
// Longer is not safer. Every second of it is a second in which a key the
// provider has just rotated to cannot be fetched, and a token signed by that key
// is a sign-in that fails — this was measured against a real Keycloak rotation,
// where a one-minute interval turned a rotation into a minute of refused
// sign-ins. The limit exists to keep an attacker from making this server fetch
// *unboundedly*, and five seconds does that: with the singleflight above
// collapsing bursts, a stream of tokens signed by keys that do not exist costs
// the provider twelve requests a minute however many are sent, which is less
// traffic than a health check.
//
// A rotation is therefore invisible in the ordinary case — providers publish a
// new key before they sign with it, so the fetch happens on the first token
// nobody has seen the key for — and costs at most one retry in the worst case.
const defaultRefetchInterval = 5 * time.Second

// errRefetchTooSoon is the rate limit refusing. It is wrapped into the
// verification failure rather than reported separately: from the caller's side
// this is a token that could not be verified, which is what it is.
var errRefetchTooSoon = errors.New("the key set was fetched too recently")

// keySet verifies ID token signatures against a provider's JWKS, caching the
// keys and refetching them — at most once per interval — when it meets a key it
// has not seen.
//
// It satisfies go-oidc's KeySet interface, which is how it is installed into the
// verifier that does the rest of the checking.
type keySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	// interval is the minimum time between two fetches. Zero disables the limit
	// and is for tests that rotate a key and expect the next request to see it.
	interval time.Duration

	// fetch collapses concurrent refetches into one request. Ten sign-ins
	// arriving in the second after a rotation should cost the provider one
	// request, not ten.
	fetch singleflight.Group

	mu      sync.RWMutex
	keys    []jose.JSONWebKey
	fetched time.Time
	// attempted is when a fetch was last *started*, successful or not. The limit
	// is on attempts rather than successes, or a provider answering 500 would be
	// asked again as fast as tokens arrived.
	attempted time.Time
}

func newKeySet(jwksURL string, client *http.Client, interval time.Duration, now func() time.Time) *keySet {
	return &keySet{url: jwksURL, client: client, interval: interval, now: now}
}

// VerifySignature checks a signed JWT against the provider's keys and returns
// its payload.
//
// It must not be called directly to verify an ID token — go-oidc's verifier is
// what checks the claims, and this only checks the signature. It is exported to
// that package through the KeySet interface and to nothing else.
func (k *keySet) VerifySignature(ctx context.Context, token string) ([]byte, error) {
	signed, err := jose.ParseSigned(token, signingAlgorithms)
	if err != nil {
		return nil, fmt.Errorf("oidc: the ID token is not a JWT signed with a supported algorithm: %w", err)
	}
	if len(signed.Signatures) != 1 {
		// Multiple signatures are legal JWS and are not something an ID token
		// does. Verifying the one that happens to be first would be choosing
		// which of two claims to believe.
		return nil, fmt.Errorf("oidc: the ID token carries %d signatures, want exactly 1",
			len(signed.Signatures))
	}
	keyID := signed.Signatures[0].Header.KeyID

	if payload, ok := verifyWith(signed, k.cached(), keyID); ok {
		return payload, nil
	}

	// An unknown key is the expected shape of a rotation: the provider signed
	// with a key published after the last fetch. It is also the shape of a forged
	// token, which is why the fetch below is rate limited and why a failure here
	// is still just a failed verification.
	refreshed, err := k.refetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc: no cached key verifies the ID token (kid %q) and the key set "+
			"could not be refetched: %w", keyID, err)
	}
	if payload, ok := verifyWith(signed, refreshed, keyID); ok {
		return payload, nil
	}
	return nil, fmt.Errorf("oidc: the ID token's signature does not verify against any key the "+
		"provider publishes (kid %q)", keyID)
}

// verifyWith tries the keys whose identifier matches, and reports whether one
// of them verified the signature.
//
// A token with no `kid` is tried against every key, which is what the
// specification requires of a verifier and what a provider publishing a single
// key relies on.
func verifyWith(signed *jose.JSONWebSignature, keys []jose.JSONWebKey, keyID string) ([]byte, bool) {
	for _, key := range keys {
		if keyID != "" && key.KeyID != keyID {
			continue
		}
		if payload, err := signed.Verify(&key); err == nil {
			return payload, true
		}
	}
	return nil, false
}

func (k *keySet) cached() []jose.JSONWebKey {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.keys
}

// refetch fetches the key set, unless it was fetched too recently.
func (k *keySet) refetch(ctx context.Context) ([]jose.JSONWebKey, error) {
	k.mu.Lock()
	if since := k.now().Sub(k.attempted); !k.attempted.IsZero() && since < k.interval {
		k.mu.Unlock()
		return nil, fmt.Errorf("%w: %s ago, and the minimum interval is %s",
			errRefetchTooSoon, since.Round(time.Second), k.interval)
	}
	k.attempted = k.now()
	k.mu.Unlock()

	// The singleflight key is a constant: there is one key set, and every caller
	// waiting here wants the same fetch.
	fetched, err, _ := k.fetch.Do("jwks", func() (any, error) {
		keys, err := k.load(ctx)
		if err != nil {
			return nil, err
		}
		k.mu.Lock()
		defer k.mu.Unlock()
		// Replaced, not merged: a key the provider has stopped publishing is a
		// key that must stop verifying tokens, which is half of what a rotation
		// is for.
		k.keys, k.fetched = keys, k.now()
		return keys, nil
	})
	if err != nil {
		return nil, err
	}
	keys, ok := fetched.([]jose.JSONWebKey)
	if !ok {
		// Unreachable: the function above returns this type or an error. Answered
		// rather than asserted, because a panic here would be a panic inside a
		// sign-in.
		return nil, fmt.Errorf("oidc: the key set fetch returned %T", fetched)
	}
	return keys, nil
}

// load performs the request. It sends `Cache-Control: no-cache` because the one
// time this runs is when the cache was not enough.
func (k *keySet) load(ctx context.Context) ([]jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build the JWKS request: %w", err)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "application/json")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", k.url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: the provider answered %s", k.url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", k.url, err)
	}

	var set jose.JSONWebKeySet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decode %s: %w", k.url, err)
	}
	if len(set.Keys) == 0 {
		// Refused rather than cached: an empty set would replace working keys
		// with none and then sit behind the rate limit for a minute.
		return nil, fmt.Errorf("%s published no keys", k.url)
	}
	return set.Keys, nil
}
