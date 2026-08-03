package servicetoken_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/servicetoken"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The token itself, and the manager over a store held in memory. What a token
// reaches is internal/authz's and internal/httpapi's; what is tested here is
// the credential — its shape, its redaction, and the five ways of not being one.

// testKey is 32 bytes, which is the minimum [servicetoken.NewHasher] accepts.
var testKey = []byte("a test encryption key, 32 bytes.")

const owner = "0192f1a0-0000-7000-8000-000000000001"

// memStore is the store, in a map. A real DuckDB is exercised by
// internal/store/identity's tests; what these need is a store that can be made
// to fail on demand, which a database cannot be asked for politely.
type memStore struct {
	mu       sync.Mutex
	byID     map[string]identity.ServiceToken
	byPrefix map[string]string

	// failWith is returned by every method when set, which is how "the database
	// is down" is told apart from "you are not authenticated".
	failWith error

	// touches counts the SetLastUsedAt calls that reached the store, which is
	// the only way to observe a debounce.
	touches int
}

func newStore() *memStore {
	return &memStore{byID: map[string]identity.ServiceToken{}, byPrefix: map[string]string{}}
}

func (s *memStore) Create(_ context.Context, in identity.NewServiceToken) (identity.ServiceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return identity.ServiceToken{}, s.failWith
	}

	token := identity.ServiceToken{
		ID:           fmt.Sprintf("token-%d", len(s.byID)+1),
		Name:         in.Name,
		Prefix:       in.Prefix,
		TokenHash:    in.TokenHash,
		OwnerUserID:  in.OwnerUserID,
		CreatedBy:    in.CreatedBy,
		Scopes:       in.Scopes,
		EngagementID: in.EngagementID,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    in.ExpiresAt.UTC(),
	}
	s.byID[token.ID] = token
	s.byPrefix[token.Prefix] = token.ID
	return token, nil
}

func (s *memStore) ByPrefix(_ context.Context, prefix string) (identity.ServiceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return identity.ServiceToken{}, s.failWith
	}

	id, ok := s.byPrefix[prefix]
	if !ok {
		return identity.ServiceToken{}, apierr.NotFound("service token", "(prefix)")
	}
	return s.byID[id], nil
}

func (s *memStore) ListByOwner(_ context.Context, ownerUserID string) ([]identity.ServiceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return nil, s.failWith
	}

	var out []identity.ServiceToken
	for _, token := range s.byID {
		if token.OwnerUserID == ownerUserID {
			out = append(out, token)
		}
	}
	return out, nil
}

func (s *memStore) Revoke(_ context.Context, id, ownerUserID string, at time.Time) (identity.ServiceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return identity.ServiceToken{}, s.failWith
	}

	token, ok := s.byID[id]
	if !ok || token.OwnerUserID != ownerUserID {
		return identity.ServiceToken{}, apierr.NotFound("service token", id)
	}
	if token.RevokedAt.IsZero() {
		token.RevokedAt = at.UTC()
		s.byID[id] = token
	}
	return token, nil
}

func (s *memStore) SetLastUsedAt(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return s.failWith
	}

	s.touches++
	token := s.byID[id]
	token.LastUsedAt = at.UTC()
	s.byID[id] = token
	return nil
}

func (s *memStore) touchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.touches
}

// revokeDirectly ends a token behind the manager's back, which is how a token
// revoked between two requests is reached.
func (s *memStore) revokeDirectly(id string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.byID[id]
	token.RevokedAt = at.UTC()
	s.byID[id] = token
}

// clock is a settable time source, so that expiry and the debounce can be
// reached without sleeping.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) read() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newManager builds a manager whose background work runs inline, so that an
// assertion about last_used_at is not racing a goroutine.
func newManager(t *testing.T, store servicetoken.Store, adjust ...func(*servicetoken.Options)) (
	*servicetoken.Manager, *clock) {
	t.Helper()

	hasher, err := servicetoken.NewHasher(testKey)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	opts := servicetoken.Options{
		Hasher:     hasher,
		Now:        c.read,
		Background: func(job func()) { job() },
		Log:        slog.New(slog.DiscardHandler),
	}
	for _, fn := range adjust {
		fn(&opts)
	}

	manager, err := servicetoken.New(store, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager, c
}

// issue mints a plausible token, failing the test if it cannot.
func issue(t *testing.T, m *servicetoken.Manager, adjust ...func(*servicetoken.NewToken)) servicetoken.Issued {
	t.Helper()

	in := servicetoken.NewToken{
		Name:        "nightly export",
		OwnerUserID: owner,
		CreatedBy:   owner,
		Scopes:      []string{string(authz.TokenScopeEngagementsRead)},
		ExpiresAt:   time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	for _, fn := range adjust {
		fn(&in)
	}

	issued, err := m.Issue(t.Context(), in)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return issued
}

// --- The credential ----------------------------------------------------------

// TestAMintedTokenHasTheDocumentedShape. The marker is what a secret scanner
// finds, and the prefix is what the row is looked up by; both are part of the
// contract with the outside world rather than an implementation detail.
func TestAMintedTokenHasTheDocumentedShape(t *testing.T) {
	t.Parallel()

	m, _ := newManager(t, newStore())
	issued := issue(t, m)

	raw := issued.Token.Reveal()
	parts := strings.Split(raw, "_")
	switch {
	case len(parts) != 3:
		t.Fatalf("a minted token has %d underscore-separated parts, want 3: %q", len(parts), raw)
	case parts[0] != servicetoken.Marker:
		t.Errorf("a minted token begins %q, want %q", parts[0], servicetoken.Marker)
	case parts[1] != issued.ServiceToken.Prefix:
		t.Errorf("the token carries the prefix %q and the row stores %q", parts[1], issued.ServiceToken.Prefix)
	}

	// The secret is not stored anywhere, in any form a lookup could use.
	if strings.Contains(issued.ServiceToken.TokenHash, parts[2]) {
		t.Error("the stored hash contains the secret")
	}
}

// TestTwoTokensShareNothing: minting is independent draws, so holding one says
// nothing about the next.
func TestTwoTokensShareNothing(t *testing.T) {
	t.Parallel()

	m, _ := newManager(t, newStore())
	first, second := issue(t, m), issue(t, m)

	switch {
	case first.Token.Reveal() == second.Token.Reveal():
		t.Error("two tokens minted in a row are identical")
	case first.ServiceToken.Prefix == second.ServiceToken.Prefix:
		t.Error("two tokens minted in a row share a prefix")
	case first.ServiceToken.TokenHash == second.ServiceToken.TokenHash:
		t.Error("two tokens minted in a row share a hash")
	}
}

// TestATokenRendersAsRedactedHoweverItIsPrinted is the property that keeps the
// secret out of the logs: not a rule anybody has to remember, but the only
// thing the type can do when asked to render itself.
func TestATokenRendersAsRedactedHoweverItIsPrinted(t *testing.T) {
	t.Parallel()

	m, _ := newManager(t, newStore())
	token := issue(t, m).Token
	secret := token.Reveal()

	renderings := map[string]string{
		"%s":       fmt.Sprintf("%s", token), //nolint:gosimple // the point is that %s is covered.
		"%q":       fmt.Sprintf("%q", token),
		"%v":       fmt.Sprintf("%v", token),
		"%#v":      fmt.Sprintf("%#v", token),
		"%x":       fmt.Sprintf("%x", token),
		"Sprint":   fmt.Sprint(token),
		"String":   token.String(),
		"in error": fmt.Errorf("wrapping %v", token).Error(),
	}
	for how, rendered := range renderings {
		if strings.Contains(rendered, secret) {
			t.Errorf("%s rendered the secret: %s", how, rendered)
		}
	}

	encoded, err := json.Marshal(struct {
		Token servicetoken.Token `json:"token"`
	}{token})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Errorf("JSON encoding carried the secret: %s", encoded)
	}

	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("token", slog.Any("token", token))
	if strings.Contains(logged.String(), secret) {
		t.Errorf("a log line carried the secret: %s", logged.String())
	}
}

// --- Resolving ---------------------------------------------------------------

func TestAValidTokenResolvesToItsRow(t *testing.T) {
	t.Parallel()

	m, _ := newManager(t, newStore())
	issued := issue(t, m)

	found, err := m.Resolve(t.Context(), issued.Token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if found.ID != issued.ServiceToken.ID {
		t.Errorf("Resolve returned %s, want %s", found.ID, issued.ServiceToken.ID)
	}
}

// TestEveryWayOfNotBeingAToken is the middleware table M1-011 asks for, at the
// layer that decides it. Every row is [servicetoken.ErrNoToken] and no row is
// anything else: a caller must not be able to tell a guessed prefix from a
// wrong secret from a revoked token.
func TestEveryWayOfNotBeingAToken(t *testing.T) {
	t.Parallel()

	store := newStore()
	m, c := newManager(t, store)

	valid := issue(t, m)
	expired := issue(t, m, func(in *servicetoken.NewToken) {
		in.ExpiresAt = c.read().Add(time.Hour)
	})
	revoked := issue(t, m)
	store.revokeDirectly(revoked.ServiceToken.ID, c.read())

	// The secret of one token against the prefix of another: a well-formed
	// value that names a real row and does not prove it.
	wrongSecret := servicetoken.Token(strings.Replace(valid.Token.Reveal(),
		valid.ServiceToken.Prefix, revoked.ServiceToken.Prefix, 1))

	c.advance(2 * time.Hour) // past the expiring one, nothing else.

	for name, presented := range map[string]servicetoken.Token{
		"empty":            "",
		"not a token":      "hello",
		"no marker":        servicetoken.Token(strings.TrimPrefix(valid.Token.Reveal(), "bl")),
		"wrong marker":     servicetoken.Token("gh" + strings.TrimPrefix(valid.Token.Reveal(), "bl")),
		"truncated secret": servicetoken.Token(valid.Token.Reveal()[:len(valid.Token.Reveal())-1]),
		"unknown prefix":   "bl_AAAAAAAAAA_" + servicetoken.Token(strings.Split(valid.Token.Reveal(), "_")[2]),
		"wrong secret":     wrongSecret,
		"expired":          expired.Token,
		"revoked":          revoked.Token,
	} {
		_, err := m.Resolve(t.Context(), presented)
		if !errors.Is(err, servicetoken.ErrNoToken) {
			t.Errorf("Resolve(%s) = %v, want ErrNoToken", name, err)
		}
	}

	// And the one that still works, so the table above is refusing things
	// rather than refusing everything.
	if _, err := m.Resolve(t.Context(), valid.Token); err != nil {
		t.Errorf("Resolve(valid) = %v, want the token", err)
	}
}

// TestAMalformedTokenCostsNoQuery: a value that cannot be a token is refused
// before the store is asked, so a caller sending rubbish cannot make this
// server work.
func TestAMalformedTokenCostsNoQuery(t *testing.T) {
	t.Parallel()

	store := newStore()
	store.failWith = errors.New("the database must not be reached")
	m, _ := newManager(t, store)

	if _, err := m.Resolve(t.Context(), "not a token"); !errors.Is(err, servicetoken.ErrNoToken) {
		t.Errorf("Resolve = %v, want ErrNoToken without touching the store", err)
	}
}

// TestADatabaseFailureIsNotAFailedAuthentication. Reporting one as the other
// would tell every integration in the deployment that its credentials had
// stopped working, at the moment somebody is already dealing with an outage.
func TestADatabaseFailureIsNotAFailedAuthentication(t *testing.T) {
	t.Parallel()

	store := newStore()
	m, _ := newManager(t, store)
	issued := issue(t, m)

	store.mu.Lock()
	store.failWith = errors.New("disk on fire")
	store.mu.Unlock()

	_, err := m.Resolve(t.Context(), issued.Token)
	switch {
	case err == nil:
		t.Fatal("Resolve succeeded while the store was failing")
	case errors.Is(err, servicetoken.ErrNoToken):
		t.Errorf("a store failure was reported as a failed authentication: %v", err)
	}
}

// --- Recording a use ---------------------------------------------------------

// TestAUseIsRecordedOncePerInterval is the debounce. The alternative is a
// serialized write behind every request an integration makes.
func TestAUseIsRecordedOncePerInterval(t *testing.T) {
	t.Parallel()

	store := newStore()
	m, c := newManager(t, store)
	issued := issue(t, m)

	for range 5 {
		if _, err := m.Resolve(t.Context(), issued.Token); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
	if got := store.touchCount(); got != 1 {
		t.Errorf("five requests inside one interval wrote last_used_at %d times, want 1", got)
	}

	c.advance(2 * time.Minute)
	if _, err := m.Resolve(t.Context(), issued.Token); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := store.touchCount(); got != 2 {
		t.Errorf("a request after the interval wrote last_used_at %d times in total, want 2", got)
	}
}

// TestAFailedTouchDoesNotFailTheRequest: the credential was good, and refusing
// it because a bookkeeping column could not be written would turn a nicety into
// an outage.
func TestAFailedTouchDoesNotFailTheRequest(t *testing.T) {
	t.Parallel()

	store := newStore()
	m, _ := newManager(t, store)
	issued := issue(t, m)

	failing := &touchFailingStore{memStore: store}
	m2, _ := newManager(t, failing)

	if _, err := m2.Resolve(t.Context(), issued.Token); err != nil {
		t.Errorf("Resolve = %v; a failed last_used_at write must not refuse a good token", err)
	}
}

// touchFailingStore answers everything the way its embedded store does, except
// the one write that must never matter to a caller.
type touchFailingStore struct {
	*memStore
}

func (touchFailingStore) SetLastUsedAt(context.Context, string, time.Time) error {
	return errors.New("the write failed")
}

// --- Creating ----------------------------------------------------------------

// TestWhatCreationRefuses. Every one of these is something the caller typed, so
// every one is a field error rather than a 500 or a silent adjustment.
func TestWhatCreationRefuses(t *testing.T) {
	t.Parallel()

	m, c := newManager(t, newStore())

	for name, adjust := range map[string]func(*servicetoken.NewToken){
		"no name": func(in *servicetoken.NewToken) { in.Name = "   " },
		"a very long name": func(in *servicetoken.NewToken) {
			in.Name = strings.Repeat("x", 101)
		},
		"no scopes":    func(in *servicetoken.NewToken) { in.Scopes = nil },
		"empty scopes": func(in *servicetoken.NewToken) { in.Scopes = []string{} },
		"an invented scope": func(in *servicetoken.NewToken) {
			in.Scopes = []string{"engagements:destroy"}
		},
		"no expiry": func(in *servicetoken.NewToken) { in.ExpiresAt = time.Time{} },
		"expired at birth": func(in *servicetoken.NewToken) {
			in.ExpiresAt = c.read().Add(-time.Second)
		},
		"beyond the maximum": func(in *servicetoken.NewToken) {
			in.ExpiresAt = c.read().Add(servicetoken.MaxLifetime + time.Hour)
		},
	} {
		in := servicetoken.NewToken{
			Name:        "nightly export",
			OwnerUserID: owner,
			CreatedBy:   owner,
			Scopes:      []string{string(authz.TokenScopeEngagementsRead)},
			ExpiresAt:   c.read().Add(24 * time.Hour),
		}
		adjust(&in)

		_, err := m.Issue(t.Context(), in)
		if !errors.Is(err, apierr.ErrValidation) {
			t.Errorf("Issue(%s) = %v, want a validation failure", name, err)
		}
	}
}

// TestARepeatedScopeIsStoredOnce: a list somebody assembled from two sources is
// not an error, and storing the repeat would render as a duplicate for the rest
// of the token's life.
func TestARepeatedScopeIsStoredOnce(t *testing.T) {
	t.Parallel()

	m, _ := newManager(t, newStore())
	issued := issue(t, m, func(in *servicetoken.NewToken) {
		in.Scopes = []string{
			string(authz.TokenScopeContentRead),
			string(authz.TokenScopeEngagementsRead),
			string(authz.TokenScopeContentRead),
		}
	})

	if got := len(issued.ServiceToken.Scopes); got != 2 {
		t.Errorf("a list with one repeat stored %d scopes, want 2: %v", got, issued.ServiceToken.Scopes)
	}
}

// TestTheMaximumLifetimeIsEnforcedAtTheBoundary, because "a year" and "a year
// and a second" are the two values anybody will actually send.
func TestTheMaximumLifetimeIsEnforcedAtTheBoundary(t *testing.T) {
	t.Parallel()

	m, c := newManager(t, newStore())

	if _, err := m.Issue(t.Context(), servicetoken.NewToken{
		Name: "exactly a year", OwnerUserID: owner, CreatedBy: owner,
		Scopes:    []string{string(authz.TokenScopeContentRead)},
		ExpiresAt: c.read().Add(servicetoken.MaxLifetime),
	}); err != nil {
		t.Errorf("a token expiring exactly at the maximum was refused: %v", err)
	}

	if _, err := m.Issue(t.Context(), servicetoken.NewToken{
		Name: "a second too long", OwnerUserID: owner, CreatedBy: owner,
		Scopes:    []string{string(authz.TokenScopeContentRead)},
		ExpiresAt: c.read().Add(servicetoken.MaxLifetime + time.Second),
	}); !errors.Is(err, apierr.ErrValidation) {
		t.Errorf("a token expiring a second past the maximum = %v, want a validation failure", err)
	}
}

// --- Construction ------------------------------------------------------------

func TestAManagerRefusesToBeBuiltWithoutWhatItNeeds(t *testing.T) {
	t.Parallel()

	hasher, err := servicetoken.NewHasher(testKey)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}

	if _, err := servicetoken.New(nil, servicetoken.Options{Hasher: hasher}); err == nil {
		t.Error("New with no store returned a manager")
	}
	if _, err := servicetoken.New(newStore(), servicetoken.Options{}); err == nil {
		t.Error("New with no hasher returned a manager; it could not check a presented token")
	}
	if _, err := servicetoken.NewHasher([]byte("too short")); err == nil {
		t.Error("NewHasher accepted key material short enough to guess")
	}
}
