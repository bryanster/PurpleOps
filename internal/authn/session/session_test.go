package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/purpleops/api"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store/identity"
	"github.com/bryanster/purpleops/internal/store/storetest"
)

// These tests are white-box (package session) because two of the things worth
// asserting — that the stored value is a keyed hash, and that the clock is the
// only thing making a session expire — are not visible from outside.

const testUserEmail = "alice@example.com"

// clock is a time source a test moves by hand, so that expiry and idleness are
// reached exactly rather than by sleeping.
type clock struct{ at time.Time }

func (c *clock) now() time.Time      { return c.at }
func (c *clock) add(d time.Duration) { c.at = c.at.Add(d) }

// newTestManager builds a Manager over a real temporary DuckDB, with sensible
// options that a caller may adjust.
func newTestManager(t *testing.T, adjust ...func(*Options)) *Manager {
	t.Helper()

	opts := Options{
		Secret:      []byte("a-test-secret-of-at-least-32-byt"),
		Lifetime:    12 * time.Hour,
		IdleTimeout: 2 * time.Hour,
		Secure:      true,
	}
	for _, fn := range adjust {
		fn(&opts)
	}

	manager, err := New(newTestStore(t), opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager
}

// newTestStore returns the session repository over a migrated database.
func newTestStore(t *testing.T) Store {
	t.Helper()
	return identity.NewSessions(storetest.Migrated(t))
}

// newManagerWithUser is the usual setup: a manager, and a user to issue
// sessions to.
func newManagerWithUser(t *testing.T, adjust ...func(*Options)) (*Manager, string) {
	t.Helper()

	db := storetest.Migrated(t)
	user, err := identity.NewUsers(db).Create(context.Background(), identity.NewUser{
		Email:        testUserEmail,
		DisplayName:  "Alice",
		PlatformRole: identity.PlatformRoleAdmin,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("creating the test user: %v", err)
	}

	opts := Options{
		Secret:      []byte("a-test-secret-of-at-least-32-byt"),
		Lifetime:    12 * time.Hour,
		IdleTimeout: 2 * time.Hour,
		Secure:      true,
	}
	for _, fn := range adjust {
		fn(&opts)
	}
	manager, err := New(identity.NewSessions(db), opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager, user.ID
}

func TestAnIssuedSessionResolvesAndStoresOnlyAHash(t *testing.T) {
	t.Parallel()

	manager, userID := newManagerWithUser(t)
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{IP: "10.0.0.1", UserAgent: "curl"}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if issued.Session.TokenHash == issued.Token.Reveal() {
		t.Error("the token itself was stored; a copy of the database would be a set of live sessions")
	}
	if issued.Session.TokenHash != manager.hash(issued.Token) {
		t.Error("what was stored is not the keyed hash of the token")
	}
	if got, want := issued.Session.IP, "10.0.0.1"; got != want {
		t.Errorf("IP = %q, want %q", got, want)
	}

	resolved, err := manager.Resolve(ctx, issued.Token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != issued.Session.ID {
		t.Errorf("Resolve returned session %q, want %q", resolved.ID, issued.Session.ID)
	}
}

func TestResolveRefusesEverythingThatIsNotALiveSession(t *testing.T) {
	t.Parallel()

	tests := map[string]func(t *testing.T) (*Manager, Token){
		"no cookie at all": func(t *testing.T) (*Manager, Token) {
			manager, _ := newManagerWithUser(t)
			return manager, ""
		},
		"a token of the wrong shape": func(t *testing.T) (*Manager, Token) {
			manager, _ := newManagerWithUser(t)
			return manager, "not-a-token"
		},
		"a well-formed token nobody holds": func(t *testing.T) (*Manager, Token) {
			manager, _ := newManagerWithUser(t)
			token, err := newToken()
			if err != nil {
				t.Fatal(err)
			}
			return manager, token
		},
		"a revoked session": func(t *testing.T) (*Manager, Token) {
			manager, userID := newManagerWithUser(t)
			issued, err := manager.Issue(t.Context(), userID, Request{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Revoke(t.Context(), issued.Session.ID); err != nil {
				t.Fatal(err)
			}
			return manager, issued.Token
		},
		"a session past its absolute expiry": func(t *testing.T) (*Manager, Token) {
			at := &clock{at: time.Now()}
			manager, userID := newManagerWithUser(t, func(o *Options) { o.Now = at.now })
			issued, err := manager.Issue(t.Context(), userID, Request{}, false)
			if err != nil {
				t.Fatal(err)
			}
			// Past the lifetime, but seen a moment ago: only the absolute expiry
			// can be what ends this one.
			at.add(13 * time.Hour)
			return manager, issued.Token
		},
		"a session that has been idle too long": func(t *testing.T) (*Manager, Token) {
			at := &clock{at: time.Now()}
			manager, userID := newManagerWithUser(t, func(o *Options) {
				o.Now = at.now
				o.Lifetime = 100 * time.Hour // so the absolute expiry is not what fires
				o.IdleTimeout = time.Hour
			})
			issued, err := manager.Issue(t.Context(), userID, Request{}, false)
			if err != nil {
				t.Fatal(err)
			}
			at.add(90 * time.Minute)
			return manager, issued.Token
		},
	}

	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			manager, token := setup(t)
			_, err := manager.Resolve(t.Context(), token)
			if !errors.Is(err, ErrNoSession) {
				t.Fatalf("Resolve() = %v, want ErrNoSession", err)
			}
		})
	}
}

// TestUsingASessionKeepsItAlive is the other half of the idle timeout: it counts
// from the last use, not from the last write.
func TestUsingASessionKeepsItAlive(t *testing.T) {
	t.Parallel()

	at := &clock{at: time.Now()}
	manager, userID := newManagerWithUser(t, func(o *Options) {
		o.Now = at.now
		o.IdleTimeout = time.Hour
	})
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Three quarters of the way to the timeout, twice over. Without the touch
	// the second call would be past it.
	for range 2 {
		at.add(45 * time.Minute)
		if _, err := manager.Resolve(ctx, issued.Token); err != nil {
			t.Fatalf("Resolve after %s of use: %v", 45*time.Minute, err)
		}
	}
}

// TestABusySessionIsNotWrittenOnEveryRequest: last_seen_at is a column whose
// only consumer is a timeout measured in hours, and every write takes the
// serialized writer's lock.
func TestABusySessionIsNotWrittenOnEveryRequest(t *testing.T) {
	t.Parallel()

	at := &clock{at: time.Now()}
	manager, userID := newManagerWithUser(t, func(o *Options) { o.Now = at.now })
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	at.add(touchInterval / 4)
	resolved, err := manager.Resolve(ctx, issued.Token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved.LastSeenAt.Equal(issued.Session.LastSeenAt) {
		t.Errorf("last_seen_at moved after %s; it should only be written every %s",
			touchInterval/4, touchInterval)
	}

	at.add(touchInterval)
	resolved, err = manager.Resolve(ctx, issued.Token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved.LastSeenAt.After(issued.Session.LastSeenAt) {
		t.Error("last_seen_at was not written after the interval had passed")
	}
}

// TestRotationKeepsTheSessionAndReplacesTheToken is the session-fixation
// defence, stated as a test: the old token stops working, the new one works, and
// it is the same session with the same absolute expiry.
func TestRotationKeepsTheSessionAndReplacesTheToken(t *testing.T) {
	t.Parallel()

	manager, userID := newManagerWithUser(t)
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := manager.Resolve(ctx, issued.Token); err != nil {
		t.Fatalf("Resolve before rotation: %v", err)
	}

	rotated, err := manager.Rotate(ctx, issued.Session.ID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if rotated.Token == issued.Token {
		t.Fatal("Rotate handed back the same token")
	}
	if rotated.Session.ID != issued.Session.ID {
		t.Errorf("Rotate made a new session %q, want the same one %q",
			rotated.Session.ID, issued.Session.ID)
	}
	if !rotated.Session.ExpiresAt.Equal(issued.Session.ExpiresAt) {
		t.Errorf("the absolute expiry moved from %s to %s; rotation is not a way to stay signed in forever",
			issued.Session.ExpiresAt, rotated.Session.ExpiresAt)
	}

	if _, err := manager.Resolve(ctx, issued.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the old token still resolves: %v", err)
	}
	if _, err := manager.Resolve(ctx, rotated.Token); err != nil {
		t.Errorf("the new token does not resolve: %v", err)
	}
}

func TestRotatingARevokedSessionIsRefused(t *testing.T) {
	t.Parallel()

	manager, userID := newManagerWithUser(t)
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := manager.Revoke(ctx, issued.Session.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := manager.Rotate(ctx, issued.Session.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Rotate() on a revoked session = %v, want a not-found — "+
			"rotating one would hand out a live token for a session that has ended", err)
	}
}

func TestRevokeOthersLeavesTheCallersSession(t *testing.T) {
	t.Parallel()

	manager, userID := newManagerWithUser(t)
	ctx := t.Context()

	keep, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	var others []Issued
	for range 2 {
		issued, err := manager.Issue(ctx, userID, Request{}, false)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		others = append(others, issued)
	}

	revoked, err := manager.RevokeOthers(ctx, userID, keep.Session.ID)
	if err != nil {
		t.Fatalf("RevokeOthers: %v", err)
	}
	if revoked != int64(len(others)) {
		t.Errorf("RevokeOthers revoked %d sessions, want %d", revoked, len(others))
	}
	if _, err := manager.Resolve(ctx, keep.Token); err != nil {
		t.Errorf("the caller's own session was revoked: %v", err)
	}
	for i, other := range others {
		if _, err := manager.Resolve(ctx, other.Token); !errors.Is(err, ErrNoSession) {
			t.Errorf("session %d still resolves after RevokeOthers: %v", i, err)
		}
	}
}

func TestRevokingTwiceIsNotAnError(t *testing.T) {
	t.Parallel()

	manager, userID := newManagerWithUser(t)
	ctx := t.Context()

	issued, err := manager.Issue(ctx, userID, Request{}, false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	for i := range 2 {
		if err := manager.Revoke(ctx, issued.Session.ID); err != nil {
			t.Fatalf("Revoke %d: %v", i+1, err)
		}
	}
}

func TestNewRefusesOptionsThatCannotProduceAUsableSession(t *testing.T) {
	t.Parallel()

	valid := Options{
		Secret:      []byte("a-test-secret-of-at-least-32-byt"),
		Lifetime:    12 * time.Hour,
		IdleTimeout: 2 * time.Hour,
	}

	tests := map[string]func(*Options){
		"no secret":                         func(o *Options) { o.Secret = nil },
		"no lifetime":                       func(o *Options) { o.Lifetime = 0 },
		"no idle timeout":                   func(o *Options) { o.IdleTimeout = 0 },
		"an idle timeout past the lifetime": func(o *Options) { o.IdleTimeout = o.Lifetime + time.Hour },
	}
	for name, break_ := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := valid
			break_(&opts)
			if _, err := New(newTestStore(t), opts); err == nil {
				t.Error("New() = nil, want an error naming what is unusable about these options")
			}
		})
	}

	if _, err := New(nil, valid); err == nil {
		t.Error("New() with no store = nil, want an error")
	}
}

// TestTheCookieCarriesTheAttributesThatProtectIt. Each of these is load-bearing;
// the comment on Cookie says which attack each one is about.
func TestTheCookieCarriesTheAttributesThatProtectIt(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	cookie := manager.Cookie("a-token", expires)

	switch {
	case cookie.Name != CookieName:
		t.Errorf("Name = %q, want %q", cookie.Name, CookieName)
	case !cookie.HttpOnly:
		t.Error("the cookie is not HttpOnly; script could read the session token")
	case !cookie.Secure:
		t.Error("the cookie is not Secure on a deployment that is not development")
	case cookie.SameSite != http.SameSiteStrictMode:
		t.Errorf("SameSite = %v, want Strict", cookie.SameSite)
	case cookie.Path != "/":
		t.Errorf("Path = %q, want %q", cookie.Path, "/")
	case cookie.Domain != "":
		t.Errorf("Domain = %q, want empty — a domain would widen the cookie to every subdomain",
			cookie.Domain)
	case !cookie.Expires.Equal(expires.UTC()):
		t.Errorf("Expires = %s, want the session's own expiry %s", cookie.Expires, expires.UTC())
	}
}

func TestOnlyDevelopmentDropsTheSecureAttribute(t *testing.T) {
	t.Parallel()

	insecure := newTestManager(t, func(o *Options) { o.Secure = false })
	if insecure.Cookie("a-token", time.Now().Add(time.Hour)).Secure {
		t.Error("Secure was set although the deployment asked for it not to be")
	}
	if insecure.ClearCookie().Secure {
		t.Error("the clearing cookie disagrees with the one it has to match")
	}
}

// TestTheClearingCookieMatchesTheOneItRemoves: a browser treats a cookie with
// different attributes as a different cookie and keeps the original.
func TestTheClearingCookieMatchesTheOneItRemoves(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	live := manager.Cookie("a-token", time.Now().Add(time.Hour))
	cleared := manager.ClearCookie()

	switch {
	case cleared.Name != live.Name:
		t.Errorf("Name = %q, want %q", cleared.Name, live.Name)
	case cleared.Path != live.Path:
		t.Errorf("Path = %q, want %q", cleared.Path, live.Path)
	case cleared.Secure != live.Secure || cleared.HttpOnly != live.HttpOnly:
		t.Error("Secure or HttpOnly differs from the cookie being cleared")
	case cleared.SameSite != live.SameSite:
		t.Error("SameSite differs from the cookie being cleared")
	case cleared.Value != "":
		t.Errorf("Value = %q, want empty", cleared.Value)
	case cleared.MaxAge >= 0:
		t.Errorf("MaxAge = %d, want a negative value, which is how a cookie is deleted", cleared.MaxAge)
	}
}

func TestFromRequestReadsTheCookieAndToleratesItsAbsence(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	if got := FromRequest(request); got != "" {
		t.Errorf("FromRequest() with no cookie = %q, want empty", got.Reveal())
	}

	request.AddCookie(&http.Cookie{Name: CookieName, Value: "the-token"})
	if got := FromRequest(request); got.Reveal() != "the-token" {
		t.Errorf("FromRequest() = %q, want %q", got.Reveal(), "the-token")
	}
}

// TestTheCookieNameMatchesTheSpecification ties this package to the
// `cookieSession` security scheme in api/openapi.yaml. A client generated from
// that document looks for the name it declares.
func TestTheCookieNameMatchesTheSpecification(t *testing.T) {
	t.Parallel()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	scheme := doc.Components.SecuritySchemes["cookieSession"]
	if scheme == nil || scheme.Value == nil {
		t.Fatal("the document declares no cookieSession security scheme")
	}
	if got := scheme.Value.Name; got != CookieName {
		t.Errorf("the specification calls the cookie %q and this package calls it %q", got, CookieName)
	}
	if got := scheme.Value.In; got != "cookie" {
		t.Errorf("cookieSession is declared in %q, want %q", got, "cookie")
	}
}

// TestTheDescriptionOfTheSchemeMentionsItsAttributes keeps the document honest
// about what the cookie is, since nothing else in it can express that.
func TestTheDescriptionOfTheSchemeMentionsItsAttributes(t *testing.T) {
	t.Parallel()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	description := doc.Components.SecuritySchemes["cookieSession"].Value.Description
	for _, attribute := range []string{"HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(description, attribute) {
			t.Errorf("the cookieSession description does not mention %s", attribute)
		}
	}
}
