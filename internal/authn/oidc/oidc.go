package oidc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/bryanster/blacklight/internal/authn/secrets"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
)

// The errors this package reports. Each is a different instruction to whoever is
// signing in, which is why they are distinguishable here and flattened into
// responses at the HTTP layer.
var (
	// ErrUnavailable means the provider could not be discovered: it is down,
	// unreachable, or the issuer is wrong. It is never this deployment refusing
	// somebody, and it never affects local login.
	ErrUnavailable = errors.New("oidc: the identity provider could not be reached")

	// ErrNoPendingSignIn means the callback does not belong to a sign-in this
	// browser started: no cookie, a state that does not match, or one that has
	// expired.
	ErrNoPendingSignIn = errors.New("oidc: no pending sign-in matches this callback")

	// ErrRejected means the provider answered, and what it said cannot be
	// accepted: it refused the request, the code would not exchange, or the ID
	// token failed verification.
	ErrRejected = errors.New("oidc: the provider's answer was rejected")
)

// defaultStateTTL is how long somebody has to finish signing in at the provider.
// Long enough to type a password, answer a second factor and read a consent
// screen; short enough that a pending sign-in left in a browser is not still
// completable tomorrow.
const defaultStateTTL = 15 * time.Minute

// defaultDiscoveryRetry is the minimum gap between two attempts to discover a
// provider that is not answering. It is the same idea as the JWKS rate limit:
// the endpoint that triggers this is public, so the retry has to be bounded by
// time rather than by how often somebody loads the login page.
const defaultDiscoveryRetry = 30 * time.Second

// availableWait is how long the login page's question waits for a discovery that
// is already running. Short: the answer decides whether a button is drawn, and a
// page that takes ten seconds to render because somebody else's server is
// hanging is the failure this whole endpoint exists to avoid.
const availableWait = 2 * time.Second

// discoveryTimeout bounds one discovery attempt, however patient the caller that
// triggered it is.
const discoveryTimeout = 15 * time.Second

// defaultHTTPTimeout bounds every request to the provider. Discovery happens
// while somebody waits on a page and the token exchange happens while somebody
// waits on a redirect, so neither may hang for the request timeout.
const defaultHTTPTimeout = 10 * time.Second

// CallbackPath is where the provider sends the browser back, relative to the API
// base path. It is here rather than in internal/httpapi because the redirect URI
// registered at the provider is built from it and has to match byte for byte —
// so the route and the URI are one constant.
const CallbackPath = "/auth/oidc/callback"

// Options configures a [Provider]. Build one with [OptionsFrom] and the two
// values this package cannot derive.
type Options struct {
	// Issuer, ClientID, ClientSecret, Scopes and GroupsClaim are the provider
	// registration, straight from the configuration.
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
	GroupsClaim  string

	// RoleMap turns the groups in an ID token into a platform role. It lives on
	// the provider because it is configuration about *this* provider, and it is
	// applied by internal/authn, which is what may change an account.
	RoleMap config.RoleMap

	// AutoProvision is carried for the same reason: it is a fact about how this
	// deployment treats this provider, and the layer that acts on it wants one
	// place to read it from.
	AutoProvision bool

	// RedirectURI is the absolute URL the provider sends the browser back to. It
	// is built from the configured base URL and must be registered at the
	// provider exactly as it is written here.
	RedirectURI string

	// Key is the deployment encryption key material. The pending state is sealed
	// under a key derived from it for this purpose alone.
	Key []byte

	// CookiePath scopes the state cookie, and is the OIDC endpoints' prefix:
	// nothing else has any reason to be sent it.
	CookiePath string

	// Secure sets the Secure attribute on that cookie — off only for a
	// development deployment on plain http, the same relaxation the session
	// cookie makes.
	Secure bool

	// StateTTL is how long a started sign-in may take to come back. Zero means
	// [defaultStateTTL].
	StateTTL time.Duration

	// KeyRefetchInterval is the minimum gap between two fetches of the
	// provider's key set. Zero means [defaultRefetchInterval]; a test that
	// rotates a key sets it negative to disable the limit.
	KeyRefetchInterval time.Duration

	// DiscoveryRetry is the minimum gap between two discovery attempts against a
	// provider that is not answering. Zero means [defaultDiscoveryRetry].
	DiscoveryRetry time.Duration

	// HTTPClient talks to the provider. Nil means one with
	// [defaultHTTPTimeout]; a test supplies one pointed at its own server.
	HTTPClient *http.Client

	// Log receives what a redirect cannot carry. Nil means slog.Default().
	Log *slog.Logger

	// Now reads the clock. Nil means time.Now.
	Now func() time.Time
}

// OptionsFrom derives the provider options from the process configuration.
//
// baseURL and basePath are what this package cannot know: where a browser
// reaches this deployment, and where the API is mounted under it. Together they
// are the redirect URI, which is the one value an operator also has to type into
// the provider's console.
func OptionsFrom(cfg config.Config, baseURL, basePath string) Options {
	return Options{
		Issuer:             cfg.OIDC.Issuer.String(),
		ClientID:           cfg.OIDC.ClientID,
		ClientSecret:       string(cfg.OIDC.ClientSecret.Reveal()),
		Scopes:             cfg.OIDC.Scopes.Values(),
		GroupsClaim:        cfg.OIDC.GroupsClaim,
		RoleMap:            cfg.OIDC.RoleMap,
		AutoProvision:      cfg.OIDC.AutoProvision,
		RedirectURI:        strings.TrimSuffix(baseURL, "/") + basePath + CallbackPath,
		Key:                cfg.Encryption.Key.Reveal(),
		CookiePath:         basePath + "/auth/oidc",
		Secure:             !cfg.Env.IsDevelopment(),
		StateTTL:           defaultStateTTL,
		KeyRefetchInterval: defaultRefetchInterval,
	}
}

// Provider is one configured OpenID Connect provider: what it was registered
// with, what discovery found, and the state of the sign-ins in flight against
// it. It is safe for concurrent use and there is one per process.
//
// Construct it with [New], which performs no I/O. Discovery happens in
// [Provider.Refresh], on demand, and never as a condition of starting up.
type Provider struct {
	issuer       string
	clientID     string
	clientSecret string
	scopes       []string
	groupsClaim  string
	roles        config.RoleMap
	provision    bool
	redirectURI  string

	cipher     *secrets.Cipher
	cookiePath string
	secure     bool
	stateTTL   time.Duration

	keyInterval    time.Duration
	discoveryRetry time.Duration
	client         *http.Client
	log            *slog.Logger
	now            func() time.Time

	// discovery collapses concurrent attempts into one request, so that a burst
	// of sign-ins against a provider that has not been discovered yet costs the
	// provider one round trip.
	discovery singleflight.Group

	// mu guards what discovery produced, when it was last attempted, and whether
	// an attempt is running.
	mu        sync.Mutex
	found     *discovered
	attempted time.Time
	inflight  bool
}

// discovered is what one successful discovery produced: the two endpoints a
// sign-in uses, and the verifier built over the provider's key set.
type discovered struct {
	oauth    *oauth2.Config
	verifier *coreoidc.IDTokenVerifier
}

// New returns a Provider, or an error describing an option that could not
// produce a working one. It performs no network I/O: a provider that is
// unreachable at startup is an operational condition, not a configuration error,
// and this constructor is called while the server is being built.
func New(opts Options) (*Provider, error) {
	switch {
	case strings.TrimSpace(opts.Issuer) == "":
		return nil, errors.New("oidc: no issuer; there is nothing to discover")
	case strings.TrimSpace(opts.ClientID) == "":
		return nil, errors.New("oidc: no client ID; the provider would not know who is asking")
	case strings.TrimSpace(opts.RedirectURI) == "":
		return nil, errors.New("oidc: no redirect URI; the provider would have nowhere to send anybody back to")
	case strings.TrimSpace(opts.GroupsClaim) == "":
		return nil, errors.New("oidc: no groups claim; role mapping would read nothing")
	case !strings.HasPrefix(opts.CookiePath, "/"):
		return nil, fmt.Errorf("oidc: the cookie path is %q; it must be absolute, or a browser scopes "+
			"the cookie to whatever directory happened to set it", opts.CookiePath)
	}

	// Its own derived key, not the one the TOTP secrets are under: two things
	// this server encrypts, two blast radii. internal/authn/secrets says why.
	cipher, err := secrets.NewFor(opts.Key, "oidc-state")
	if err != nil {
		return nil, fmt.Errorf("oidc: %w", err)
	}

	p := &Provider{
		issuer:         opts.Issuer,
		clientID:       opts.ClientID,
		clientSecret:   opts.ClientSecret,
		scopes:         opts.Scopes,
		groupsClaim:    opts.GroupsClaim,
		roles:          opts.RoleMap,
		provision:      opts.AutoProvision,
		redirectURI:    opts.RedirectURI,
		cipher:         cipher,
		cookiePath:     opts.CookiePath,
		secure:         opts.Secure,
		stateTTL:       opts.StateTTL,
		keyInterval:    opts.KeyRefetchInterval,
		discoveryRetry: opts.DiscoveryRetry,
		client:         opts.HTTPClient,
		log:            opts.Log,
		now:            opts.Now,
	}
	if p.stateTTL <= 0 {
		p.stateTTL = defaultStateTTL
	}
	if p.keyInterval == 0 {
		p.keyInterval = defaultRefetchInterval
	}
	if p.discoveryRetry == 0 {
		p.discoveryRetry = defaultDiscoveryRetry
	}
	if p.client == nil {
		p.client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if p.log == nil {
		p.log = slog.Default()
	}
	if p.now == nil {
		p.now = time.Now
	}
	return p, nil
}

// Issuer returns the provider's issuer identifier, for a log line or an
// operator-facing description of what this deployment is configured against.
func (p *Provider) Issuer() string { return p.issuer }

// AutoProvision reports whether an unknown person signing in through this
// provider gets an account.
func (p *Provider) AutoProvision() bool { return p.provision }

// Role returns the platform role these groups map to, and false when none of
// them is mapped. See [config.RoleMap]: the strongest match wins.
func (p *Provider) Role(groups []string) (authz.PlatformRole, bool) {
	return p.roles.Role(groups)
}

// Available reports whether a sign-in through this provider can be offered right
// now, and is what decides whether the login page draws a button.
//
// It waits for a discovery already in flight — the one the server starts at boot
// — but never for longer than [availableWait], so the login page renders at a
// predictable speed whether the provider is healthy or hanging. Once discovery
// has succeeded it costs nothing at all.
//
// False is not permanent. Discovery is retried, rate limited, so a provider that
// was down when this server booted starts being offered on its own.
func (p *Provider) Available(ctx context.Context) bool {
	if p.cached() != nil {
		return true
	}
	// The bound is on the *waiting*, not on the discovery: the attempt this joins
	// runs on a deadline of its own and finishes whatever this caller does.
	waiting, cancel := context.WithTimeout(ctx, availableWait)
	defer cancel()

	// The error is the log's, and p.ready has already written it: what this
	// answers is whether there is a button to draw.
	found, err := p.ready(waiting)
	return err == nil && found != nil
}

// Refresh performs discovery now, and is what the server calls once at startup —
// in the background, and ignoring the error beyond logging it. A provider that
// is down at boot must not stop the server from starting, because the login page
// it would have served is the one that still works.
func (p *Provider) Refresh(ctx context.Context) error {
	_, err := p.ready(ctx)
	return err
}

// ready returns the discovered provider, discovering it first if that has not
// happened yet.
//
// The three states it distinguishes, and why each is worth distinguishing:
//
//   - Discovered. No I/O, no lock held for longer than a read.
//   - An attempt is in flight. Join it. This is the common case in the second
//     after startup — the boot-time attempt is still running and somebody has
//     already loaded the login page — and refusing them because an attempt was
//     "recently made" would be refusing them *because* one is about to succeed.
//   - Nothing in flight. Start one, unless the last attempt was too recent: the
//     endpoints that reach here are public, so the retry has to be bounded by
//     time rather than by how often somebody asks.
func (p *Provider) ready(ctx context.Context) (*discovered, error) {
	p.mu.Lock()
	found, inflight, due := p.found, p.inflight, p.discoveryDue()
	p.mu.Unlock()

	switch {
	case found != nil:
		return found, nil
	case !inflight && !due:
		return nil, fmt.Errorf("%w: %s was tried too recently to try again", ErrUnavailable, p.issuer)
	}
	return p.discoverOnce(ctx)
}

// discoveryDue reports whether another discovery attempt is allowed. Callers
// hold p.mu.
func (p *Provider) discoveryDue() bool {
	return p.attempted.IsZero() || p.now().Sub(p.attempted) >= p.discoveryRetry
}

// cached returns what discovery last produced, or nil.
func (p *Provider) cached() *discovered {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.found
}

// discoverOnce runs discovery, or joins the run already happening.
//
// The attempt is detached from the caller's context and given a deadline of its
// own. Two callers with different patience — a login page that waits two seconds
// and a person who clicked the button — must not be able to cancel each other's
// work, and the first one to give up must not leave the second waiting on a
// request that has already been abandoned.
func (p *Provider) discoverOnce(ctx context.Context) (*discovered, error) {
	result := p.discovery.DoChan("discover", func() (any, error) {
		attempt, cancel := context.WithTimeout(context.WithoutCancel(ctx), discoveryTimeout)
		defer cancel()

		p.mu.Lock()
		p.attempted, p.inflight = p.now(), true
		p.mu.Unlock()

		found, err := p.discover(attempt)

		p.mu.Lock()
		p.inflight = false
		if err == nil {
			p.found = found
		}
		p.mu.Unlock()

		if err != nil {
			// Logged here rather than by every caller: one of them is a background
			// goroutine with nowhere to return an error to, and another is a
			// redirect, which cannot carry one.
			p.log.WarnContext(attempt, "the identity provider could not be discovered; "+
				"single sign-on is unavailable and local sign-in is unaffected",
				slog.String("issuer", p.issuer),
				slog.String("error", err.Error()))
			return nil, err
		}
		p.log.InfoContext(attempt, "discovered the identity provider",
			slog.String("issuer", p.issuer))
		return found, nil
	})

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %s did not answer in time: %w", ErrUnavailable, p.issuer, ctx.Err())
	case answer := <-result:
		if answer.Err != nil {
			return nil, answer.Err
		}
		found, ok := answer.Val.(*discovered)
		if !ok {
			// Unreachable, and answered rather than asserted for the reason
			// keys.go gives: this is inside somebody's sign-in.
			return nil, fmt.Errorf("%w: discovery returned %T", ErrUnavailable, answer.Val)
		}
		return found, nil
	}
}

// discover reads the provider's metadata and builds the pieces a sign-in needs.
func (p *Provider) discover(ctx context.Context) (*discovered, error) {
	// go-oidc takes its HTTP client through the context, which is not how
	// anybody would design it today. This is the one place that has to know.
	ctx = coreoidc.ClientContext(ctx, p.client)

	// NewProvider fetches /.well-known/openid-configuration and refuses a
	// document whose `issuer` is not the one asked for — which is the check that
	// makes a misconfigured or hijacked discovery URL a failure here rather than
	// a set of endpoints belonging to somebody else.
	remote, err := coreoidc.NewProvider(ctx, p.issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUnavailable, p.issuer, err)
	}

	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := remote.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("%w: %s published metadata that could not be read: %w",
			ErrUnavailable, p.issuer, err)
	}
	if metadata.JWKSURL == "" {
		return nil, fmt.Errorf("%w: %s published no jwks_uri, so no ID token could ever be verified",
			ErrUnavailable, p.issuer)
	}

	algorithms := make([]string, 0, len(signingAlgorithms))
	for _, algorithm := range signingAlgorithms {
		algorithms = append(algorithms, string(algorithm))
	}

	return &discovered{
		oauth: &oauth2.Config{
			ClientID:     p.clientID,
			ClientSecret: p.clientSecret,
			Endpoint:     remote.Endpoint(),
			RedirectURL:  p.redirectURI,
			Scopes:       p.scopes,
		},
		// Our key set rather than the provider's own, for the rate limit — see
		// keys.go. Everything else about the verification is go-oidc's:
		// signature, issuer, audience, expiry and not-before.
		verifier: coreoidc.NewVerifier(p.issuer,
			newKeySet(metadata.JWKSURL, p.client, p.keyInterval, p.now),
			&coreoidc.Config{
				ClientID:             p.clientID,
				SupportedSigningAlgs: algorithms,
				Now:                  p.now,
			}),
	}, nil
}

// Start begins a sign-in: it returns the provider URL to send the browser to,
// and the sealed state that has to come back with it.
//
// returnTo is where to land afterwards, and has already been through
// [SafeReturnTo] — this is the second place it is checked, because the value
// travels through a cookie in between and a check that only happens on the way
// in is a check that trusts what came back.
type Start struct {
	// AuthorizationURL is where the browser goes.
	AuthorizationURL string

	// Sealed is the pending state, for [Provider.Cookie].
	Sealed string
}

// Start mints the pending state and builds the authorization URL.
func (p *Provider) Start(ctx context.Context, returnTo string) (Start, error) {
	// Before discovery: what the caller sent is wrong whatever the provider's
	// state is, and answering "there is no provider" to a request that is
	// malformed would send somebody looking in the wrong place.
	safe, err := SafeReturnTo(returnTo)
	if err != nil {
		return Start{}, err
	}

	found, err := p.ready(ctx)
	if err != nil {
		return Start{}, err
	}

	state, err := newState()
	if err != nil {
		return Start{}, err
	}
	nonce, err := newState()
	if err != nil {
		return Start{}, err
	}
	// oauth2 generates a verifier of the length RFC 7636 wants and computes the
	// S256 challenge from it. Both halves come from the library, so there is no
	// chance of sending a "plain" challenge by accident — which would make PKCE
	// decorative.
	verifier := oauth2.GenerateVerifier()

	sealed, err := p.seal(pending{
		State:     state,
		Nonce:     nonce,
		Verifier:  verifier,
		ReturnTo:  safe,
		ExpiresAt: p.now().Add(p.stateTTL),
	})
	if err != nil {
		return Start{}, err
	}

	return Start{
		AuthorizationURL: found.oauth.AuthCodeURL(state,
			coreoidc.Nonce(nonce),
			oauth2.S256ChallengeOption(verifier)),
		Sealed: sealed,
	}, nil
}

// Callback is what came back from the provider: the query parameters of the
// redirect, and the sealed state out of the cookie.
type Callback struct {
	State string
	Code  string

	// Error and ErrorDescription are what the provider sends instead of a code
	// when it refuses — `access_denied` when somebody declines consent, and
	// anything else when the registration is wrong.
	Error            string
	ErrorDescription string

	// Sealed is the cookie's value, from [SealedFrom].
	Sealed string
}

// Identity is what the provider vouches for, verified. It is claims and nothing
// else: no account exists yet as far as this package is concerned, and whether
// one should is internal/authn's decision.
type Identity struct {
	// Subject is the `sub` claim: the provider's permanent identifier for this
	// person. It is what an account is linked by, because it is the only claim
	// that is promised not to change — an address does change, and linking by
	// one that has been reassigned is an account takeover.
	Subject string

	// Email and EmailVerified are the address and whether the provider says it
	// has been verified. The second is not decoration: linking a federated login
	// to an existing local account by an *unverified* address means anybody who
	// can type an address at the provider can claim that account.
	Email         string
	EmailVerified bool

	// DisplayName is the best name the token offered, and falls back to the
	// address rather than being empty.
	DisplayName string

	// Groups are the values of the configured groups claim, and are empty when
	// the provider sent none.
	Groups []string

	// ReturnTo is the path this sign-in should land on, already checked.
	ReturnTo string
}

// Complete finishes a sign-in: it checks the pending state, exchanges the
// authorization code, and verifies the ID token.
//
// The order matters and is the order below. State first, so an unsolicited
// callback — the login-CSRF case — costs nothing and never reaches the provider.
// Then the provider's own refusal, which is only worth reporting for a sign-in
// that was really started here. Then the exchange, which is where PKCE binds the
// code to this browser. Then verification, and only then the nonce, which is the
// check that this ID token was minted for this exchange.
func (p *Provider) Complete(ctx context.Context, in Callback) (Identity, error) {
	found, err := p.ready(ctx)
	if err != nil {
		return Identity{}, err
	}

	state, err := p.open(in.Sealed, in.State)
	if err != nil {
		return Identity{}, err
	}

	if in.Error != "" {
		// The description is the provider's prose and goes to the log. Repeating
		// it to the browser would mean rendering text this server did not write.
		return Identity{}, fmt.Errorf("%w: the provider refused the request: %s (%s)",
			ErrRejected, in.Error, in.ErrorDescription)
	}
	if in.Code == "" {
		return Identity{}, fmt.Errorf("%w: the callback carries neither an authorization code nor an error",
			ErrRejected)
	}

	ctx = coreoidc.ClientContext(ctx, p.client)
	token, err := found.oauth.Exchange(ctx, in.Code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("%w: the authorization code could not be exchanged: %w",
			ErrRejected, err)
	}

	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Identity{}, fmt.Errorf("%w: the token response carried no ID token, so nothing "+
			"identifies who signed in", ErrRejected)
	}

	// Signature, issuer, audience, expiry and not-before, all in the library.
	idToken, err := found.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: the ID token did not verify: %w", ErrRejected, err)
	}
	// The nonce is ours to check: the library carries the claim and has no idea
	// which sign-in this is. Constant time, like the state.
	if !constantTimeEqual(idToken.Nonce, state.Nonce) {
		return Identity{}, fmt.Errorf(
			"%w: the ID token's nonce is not the one this sign-in was started with", ErrRejected)
	}

	identity, err := p.identityOf(idToken)
	if err != nil {
		return Identity{}, err
	}
	identity.ReturnTo, err = SafeReturnTo(state.ReturnTo)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}
