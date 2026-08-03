package saml

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	crewjam "github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
	"golang.org/x/sync/singleflight"

	"github.com/bryanster/blacklight/internal/authn/returnto"
	"github.com/bryanster/blacklight/internal/authn/secrets"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
)

// The errors this package reports. They are distinguishable here and flattened
// at the HTTP layer, for the reason internal/authn/oidc gives: a caller who can
// tell which half of a forgery failed is a caller mapping the implementation.
var (
	// ErrUnavailable means the identity provider's metadata could not be read:
	// it is down, unreachable, or publishing something this is not. It is never
	// this deployment refusing somebody, and it never affects local login.
	ErrUnavailable = errors.New("saml: the identity provider's metadata could not be read")

	// ErrNoPendingSignIn means the assertion arrived with no sealed cookie
	// naming a sign-in this browser started, on a deployment that does not
	// accept identity-provider-initiated sign-in.
	ErrNoPendingSignIn = errors.New("saml: no pending sign-in matches this assertion")

	// ErrRejected means the assertion was read and cannot be accepted: the
	// signature, the issuer, the audience, the recipient, the validity window,
	// or the request it claims to answer.
	ErrRejected = errors.New("saml: the assertion was rejected")

	// ErrReplayed is [ErrRejected]'s most interesting case, and the one no
	// library catches: this exact assertion has been accepted here before.
	ErrReplayed = fmt.Errorf("%w: it has already been used", ErrRejected)
)

// The paths this service provider occupies, relative to the API base path.
//
// They are constants here rather than in internal/httpapi because two of them
// are values an identity provider administrator types into a console — the
// assertion consumer URL and the entity ID — and they are checked byte for byte
// against the assertion's `Recipient` and `Audience`. The route and the
// registered URL have to be one string or they drift.
const (
	// MetadataPath serves the service provider metadata, and doubles as the
	// default entity ID.
	MetadataPath = "/auth/saml/metadata"

	// StartPath begins a service-provider-initiated sign-in.
	StartPath = "/auth/saml/start"

	// ACSPath is the assertion consumer service: where the identity provider
	// POSTs the assertion.
	ACSPath = "/auth/saml/acs"

	// cookiePathSuffix scopes the pending-request cookie to these three
	// endpoints and nothing else.
	cookiePathSuffix = "/auth/saml"
)

// defaultStateTTL is how long somebody has to finish signing in at the identity
// provider, and how long the sealed cookie lives. The same value and the same
// argument as the OIDC one: long enough to type a password and answer a second
// factor, short enough that a sign-in abandoned in a browser is not still
// completable tomorrow.
const defaultStateTTL = 15 * time.Minute

// defaultFetchRetry is the minimum gap between two attempts to fetch metadata
// from an identity provider that is not answering. The endpoints that trigger a
// fetch are public, so the retry has to be bounded by time rather than by how
// often somebody loads the login page.
const defaultFetchRetry = 30 * time.Second

// availableWait is how long the login page's question waits for a fetch that is
// already running, and fetchTimeout bounds one attempt however patient the
// caller is. Both exist for the reason the OIDC equivalents do: whether a button
// is drawn must not depend on how slow somebody else's web server is today.
const (
	availableWait = 2 * time.Second
	fetchTimeout  = 15 * time.Second
)

// defaultHTTPTimeout bounds every request to the identity provider.
const defaultHTTPTimeout = 10 * time.Second

// maxAllowedSkew is the widest clock skew this deployment will ever be
// configured with, and it is also the reason for the init below.
//
// github.com/crewjam/saml keeps its skew and issue-delay tolerances in
// package-level variables, which is not how anybody would design it today. This
// package sets them **once, to constants**, and never from configuration —
// because a value written per [Provider] would be a global mutated at
// construction, which two providers or two parallel tests would fight over, and
// the race detector would be right about.
//
// What is set here is the *ceiling*: wide enough that the library never refuses
// an assertion this deployment might have accepted. The configured skew is then
// enforced by [Provider.checkFreshness], which runs after the library is done
// and is strictly narrower. So the effective tolerance is always
// [SAML.ClockSkew] from the configuration, the library is never the thing
// deciding it, and nothing here is mutable at runtime.
const (
	maxAllowedSkew = 5 * time.Minute

	// defaultIssueDelay is how stale a response or assertion may be by its own
	// IssueInstant. Ninety seconds is the library's default and a reasonable
	// one: it is a document that was just minted for a browser that is standing
	// on the redirect.
	defaultIssueDelay = 90 * time.Second
)

// init pins the library's two package-level tolerances to the constants above.
// See [maxAllowedSkew]: they are somebody else's globals, and setting them once
// to constants is what stops them behaving like mutable state.
//
//nolint:gochecknoinits // pinning a dependency's package variables, once, to constants
func init() {
	crewjam.MaxClockSkew = maxAllowedSkew
	crewjam.MaxIssueDelay = defaultIssueDelay + maxAllowedSkew
}

// Assertions is the replay cache: the record of which assertion IDs have
// already been accepted here. [*identity.SAMLAssertions] satisfies it.
//
// It is an interface rather than the repository because this package owns a
// protocol rule and not a table, and because the rule is worth being able to
// read in one place: Consume must fail for an ID it has seen, and the failure
// must be decided by whatever is storing it rather than by a read followed by a
// write.
type Assertions interface {
	// Consume records an assertion as used, and returns a non-nil error for one
	// that has been used before. expiresAt is the last moment it could still be
	// replayed.
	Consume(ctx context.Context, assertionID string, expiresAt time.Time) error
}

// Options configures a [Provider]. Build one with [OptionsFrom] and the values
// this package cannot derive.
type Options struct {
	// MetadataURL is where the identity provider publishes its metadata, and
	// MetadataXML is that document itself. Exactly one is set: with a URL the
	// document is fetched lazily and retried, and with XML it is parsed once
	// here and this provider needs no network at all.
	MetadataURL string
	MetadataXML []byte

	// EntityID is what this deployment calls itself, and the value every
	// assertion's `Audience` must carry.
	EntityID string

	// ACSURL is the absolute URL the identity provider POSTs the assertion to.
	// It is checked against the assertion's `Recipient` and the response's
	// `Destination`, so it must be exactly what is registered there.
	ACSURL string

	// OurMetadataURL is the absolute URL this deployment's own metadata is
	// served from, and appears in that document.
	OurMetadataURL string

	// Certificate and SigningKey are this service provider's key pair. The
	// certificate is published; the key signs authentication requests and never
	// leaves this process.
	Certificate *x509.Certificate
	SigningKey  crypto.Signer

	// EmailAttributes, NameAttributes and GroupAttributes are the assertion
	// attributes to read each fact out of, best first.
	EmailAttributes []string
	NameAttributes  []string
	GroupAttributes []string

	// RoleMap turns the groups in an assertion into a platform role. It lives
	// here for the reason the OIDC one does: it is configuration about this
	// provider, applied by internal/authn, which is what may change an account.
	RoleMap config.RoleMap

	// AutoProvision and AllowIDPInitiated are carried for the same reason —
	// facts about how this deployment treats this provider, read by the layer
	// that acts on them.
	AutoProvision     bool
	AllowIDPInitiated bool

	// ClockSkew is how far the identity provider's clock may be from this one.
	// Zero means no tolerance at all, which is a legitimate configuration; it is
	// never widened silently, and never beyond [maxAllowedSkew].
	ClockSkew time.Duration

	// Assertions is the replay cache. A Provider will not be built without one:
	// see [New].
	Assertions Assertions

	// SealKey is the deployment encryption key material. The pending request is
	// sealed under a key derived from it for this purpose alone.
	SealKey []byte

	// CookiePath scopes the pending-request cookie, and is the SAML endpoints'
	// prefix: nothing else has any reason to be sent it.
	CookiePath string

	// StateTTL is how long a started sign-in may take to come back. Zero means
	// [defaultStateTTL].
	StateTTL time.Duration

	// FetchRetry is the minimum gap between two metadata fetches against a
	// provider that is not answering. Zero means [defaultFetchRetry].
	FetchRetry time.Duration

	// HTTPClient fetches the metadata. Nil means one with
	// [defaultHTTPTimeout]; a test supplies one pointed at its own server.
	HTTPClient *http.Client

	// Log receives what a redirect cannot carry. Nil means slog.Default().
	Log *slog.Logger

	// Now reads the clock. Nil means time.Now.
	//
	// It is this package's clock only — the sealed cookie's expiry and the
	// freshness check. The library reads its own (crewjam.TimeNow), which a test
	// that needs to move time has to set as well.
	Now func() time.Time
}

// OptionsFrom derives the provider options from the process configuration,
// reading the two PEM files and, when the metadata is a file, that too.
//
// baseURL and basePath are what this package cannot know: where a browser
// reaches this deployment, and where the API is mounted under it. Together they
// are the assertion consumer URL and the default entity ID, which are the two
// values an operator also types into the identity provider's console.
//
// It reports a configuration this package cannot make a working provider out
// of. It never reports the identity provider being unreachable — see
// [Provider.Refresh].
func OptionsFrom(cfg config.Config, baseURL, basePath string) (Options, error) {
	base := strings.TrimSuffix(baseURL, "/") + basePath

	certificate, err := loadCertificate(cfg.SAML.CertFile)
	if err != nil {
		return Options{}, err
	}
	key, err := loadKey(cfg.SAML.KeyFile)
	if err != nil {
		return Options{}, err
	}

	var metadataXML []byte
	if cfg.SAML.MetadataFile != "" {
		if metadataXML, err = readMetadataFile(cfg.SAML.MetadataFile); err != nil {
			return Options{}, err
		}
	}

	entityID := cfg.SAML.EntityID
	if entityID == "" {
		// The conventional default, and the one the published document
		// advertises: a service provider is named by where its metadata lives.
		entityID = base + MetadataPath
	}

	return Options{
		MetadataURL:       cfg.SAML.MetadataURL.String(),
		MetadataXML:       metadataXML,
		EntityID:          entityID,
		ACSURL:            base + ACSPath,
		OurMetadataURL:    base + MetadataPath,
		Certificate:       certificate,
		SigningKey:        key,
		EmailAttributes:   cfg.SAML.EmailAttribute.Values(),
		NameAttributes:    cfg.SAML.NameAttribute.Values(),
		GroupAttributes:   cfg.SAML.GroupsAttribute.Values(),
		RoleMap:           cfg.SAML.RoleMap,
		AutoProvision:     cfg.SAML.AutoProvision,
		AllowIDPInitiated: cfg.SAML.AllowIDPInitiated,
		ClockSkew:         cfg.SAML.ClockSkew,
		SealKey:           cfg.Encryption.Key.Reveal(),
		CookiePath:        basePath + cookiePathSuffix,
		StateTTL:          defaultStateTTL,
	}, nil
}

// Provider is one configured SAML identity provider: how this deployment is
// registered with it, what its metadata says, and the sign-ins in flight against
// it. It is safe for concurrent use and there is one per process.
//
// Construct it with [New], which performs no network I/O. Metadata is fetched in
// [Provider.Refresh], on demand, and never as a condition of starting up.
type Provider struct {
	// template is the service provider with everything filled in except the
	// identity provider's metadata, which arrives later and is copied onto a
	// value derived from this one. It is never used for parsing directly; see
	// [Provider.consumer].
	template crewjam.ServiceProvider

	metadataURL string
	metadataXML []byte

	emailAttributes []string
	nameAttributes  []string
	groupAttributes []string

	roles        config.RoleMap
	provision    bool
	idpInitiated bool
	skew         time.Duration

	assertions Assertions

	cipher     *secrets.Cipher
	cookiePath string
	stateTTL   time.Duration

	fetchRetry time.Duration
	client     *http.Client
	log        *slog.Logger
	now        func() time.Time

	// fetching collapses concurrent attempts into one request, so a burst of
	// sign-ins against a provider whose metadata has not been read yet costs it
	// one round trip.
	fetching singleflight.Group

	// mu guards what the fetch produced, when it was last attempted, and whether
	// an attempt is running.
	mu        sync.Mutex
	found     *crewjam.EntityDescriptor
	attempted time.Time
	inflight  bool
}

// New returns a Provider, or an error describing an option that could not
// produce a working one.
//
// It performs no network I/O: an identity provider that is unreachable at
// startup is an operational condition, not a configuration error, and this
// constructor is called while the server is being built. Static metadata is
// parsed here, because a file that is not metadata *is* a configuration error
// and there is nothing to wait for.
func New(opts Options) (*Provider, error) {
	switch {
	case opts.MetadataURL == "" && len(opts.MetadataXML) == 0:
		return nil, errors.New("saml: no identity provider metadata; there is nothing to trust")
	case opts.MetadataURL != "" && len(opts.MetadataXML) > 0:
		return nil, errors.New("saml: both a metadata URL and metadata XML; there is one identity provider")
	case opts.EntityID == "":
		return nil, errors.New("saml: no entity ID; assertions would be audience-restricted to nothing")
	case opts.ACSURL == "":
		return nil, errors.New("saml: no assertion consumer URL; the provider would have nowhere to post to")
	case opts.Certificate == nil || opts.SigningKey == nil:
		return nil, errors.New("saml: no service provider key pair; authentication requests could not be signed")
	case opts.Assertions == nil:
		// Refused rather than defaulted to a no-op. A provider with no replay
		// cache accepts every assertion as many times as it is presented, which
		// is the one failure in this package that looks exactly like success.
		return nil, errors.New("saml: no assertion store; there would be no replay prevention")
	case len(opts.EmailAttributes) == 0:
		return nil, errors.New("saml: no email attributes; no assertion could ever name an account")
	case opts.ClockSkew < 0 || opts.ClockSkew > maxAllowedSkew:
		return nil, fmt.Errorf("saml: a clock skew of %s is outside 0..%s", opts.ClockSkew, maxAllowedSkew)
	case !strings.HasPrefix(opts.CookiePath, "/"):
		return nil, fmt.Errorf("saml: the cookie path is %q; it must be absolute, or a browser scopes "+
			"the cookie to whatever directory happened to set it", opts.CookiePath)
	}

	acs, err := url.Parse(opts.ACSURL)
	if err != nil {
		return nil, fmt.Errorf("saml: the assertion consumer URL %q does not parse: %w", opts.ACSURL, err)
	}
	ourMetadata, err := url.Parse(opts.OurMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("saml: the metadata URL %q does not parse: %w", opts.OurMetadataURL, err)
	}

	// Its own derived key, not the one the TOTP secrets or the OIDC state are
	// under: three things this server encrypts, three blast radii.
	// internal/authn/secrets says why.
	cipher, err := secrets.NewFor(opts.SealKey, "saml-request")
	if err != nil {
		return nil, fmt.Errorf("saml: %w", err)
	}

	p := &Provider{
		template: crewjam.ServiceProvider{
			EntityID:    opts.EntityID,
			Key:         opts.SigningKey,
			Certificate: opts.Certificate,
			AcsURL:      *acs,
			MetadataURL: *ourMetadata,
			// Signed authentication requests. Not required by the profile, and
			// worth the cost anyway: it is what lets the identity provider know
			// a request naming this deployment came from it, and every
			// commercial provider either wants it or ignores it.
			SignatureMethod: dsig.RSASHA256SignatureMethod,
			// The format that means "an identifier for this person that does not
			// change" — which is what an account is linked by. See
			// [Identity.Subject].
			AuthnNameIDFormat: crewjam.PersistentNameIDFormat,
			// AllowIDPInitiated is deliberately *not* set from configuration
			// here: it is set per assertion, on a copy, by
			// [Provider.consumer]. Setting it on the template would disable the
			// InResponseTo binding for service-provider-initiated sign-ins too,
			// which is the one thing this package would rather not give away.
		},
		metadataURL:     opts.MetadataURL,
		metadataXML:     opts.MetadataXML,
		emailAttributes: opts.EmailAttributes,
		nameAttributes:  opts.NameAttributes,
		groupAttributes: opts.GroupAttributes,
		roles:           opts.RoleMap,
		provision:       opts.AutoProvision,
		idpInitiated:    opts.AllowIDPInitiated,
		skew:            opts.ClockSkew,
		assertions:      opts.Assertions,
		cipher:          cipher,
		cookiePath:      opts.CookiePath,
		stateTTL:        opts.StateTTL,
		fetchRetry:      opts.FetchRetry,
		client:          opts.HTTPClient,
		log:             opts.Log,
		now:             opts.Now,
	}
	if p.stateTTL <= 0 {
		p.stateTTL = defaultStateTTL
	}
	if p.fetchRetry == 0 {
		p.fetchRetry = defaultFetchRetry
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

	if len(opts.MetadataXML) > 0 {
		descriptor, err := parseMetadata(opts.MetadataXML)
		if err != nil {
			return nil, err
		}
		p.found = descriptor
	}
	return p, nil
}

// EntityID returns what this deployment calls itself to the identity provider,
// for a startup log line or an operator-facing description of the registration.
func (p *Provider) EntityID() string { return p.template.EntityID }

// AutoProvision reports whether an unknown person signing in through this
// provider gets an account.
func (p *Provider) AutoProvision() bool { return p.provision }

// Role returns the platform role these groups map to, and false when none of
// them is mapped. It is [config.RoleMap]'s rule, which is the same one the OIDC
// path uses: the strongest match wins.
func (p *Provider) Role(groups []string) (authz.PlatformRole, bool) {
	return p.roles.Role(groups)
}

// Available reports whether a sign-in through this provider can be offered right
// now, and is what decides whether the login page draws a button.
//
// It waits for a fetch already in flight — the one the server starts at boot —
// but never longer than [availableWait]. False is not permanent: the fetch is
// retried, rate limited, so a provider that was down when this server booted
// starts being offered on its own.
func (p *Provider) Available(ctx context.Context) bool {
	if p.cached() != nil {
		return true
	}
	waiting, cancel := context.WithTimeout(ctx, availableWait)
	defer cancel()

	found, err := p.ready(waiting)
	return err == nil && found != nil
}

// Refresh reads the identity provider's metadata now, and is what the server
// calls once at startup — in the background, ignoring the error beyond logging
// it. A provider that is down at boot must not stop the server from starting,
// because the login page it would have served is the one that still works.
func (p *Provider) Refresh(ctx context.Context) error {
	_, err := p.ready(ctx)
	return err
}

// Metadata returns this service provider's metadata document, for the endpoint
// an identity provider administrator points their console at.
//
// It needs nothing from the identity provider, so it answers while that provider
// is unreachable — which is the moment somebody is most likely to be fetching
// it, because they are in the middle of setting the registration up.
func (p *Provider) Metadata() ([]byte, error) {
	// A copy, because Metadata() reads the whole service provider and the
	// template is shared.
	sp := p.template
	descriptor := sp.Metadata()

	trimArtifactBinding(descriptor)

	encoded, err := marshalMetadata(descriptor)
	if err != nil {
		return nil, fmt.Errorf("saml: render the service provider metadata: %w", err)
	}
	return encoded, nil
}

// Start begins a sign-in: the identity provider URL to send the browser to, and
// the sealed pending request that has to come back with it.
type Start struct {
	// RedirectURL is where the browser goes.
	RedirectURL string

	// Sealed is the pending request, for [Provider.Cookie].
	Sealed string
}

// Start mints the authentication request and seals what has to come back.
//
// returnTo has already been through [returnto.Safe] at the HTTP layer; it is
// checked here as well, because this is the value that goes into the cookie and
// a check that only happens at the edge is a check the cookie's contents skip.
func (p *Provider) Start(ctx context.Context, returnTo string) (Start, error) {
	// Before the metadata: what the caller sent is wrong whatever the identity
	// provider's state is, and answering "there is no provider" to a malformed
	// request would send somebody looking in the wrong place.
	safe, err := returnto.Safe(returnTo)
	if err != nil {
		return Start{}, err
	}

	found, err := p.ready(ctx)
	if err != nil {
		return Start{}, err
	}

	sp := p.template
	sp.IDPMetadata = found

	destination := sp.GetSSOBindingLocation(crewjam.HTTPRedirectBinding)
	if destination == "" {
		// A provider that publishes no redirect binding is one this deployment
		// cannot start a sign-in with. It is a fact about their metadata, so it
		// is [ErrUnavailable] rather than a fault here.
		return Start{}, fmt.Errorf("%w: %s publishes no HTTP-Redirect single sign-on service",
			ErrUnavailable, found.EntityID)
	}

	request, err := sp.MakeAuthenticationRequest(destination,
		crewjam.HTTPRedirectBinding, crewjam.HTTPPostBinding)
	if err != nil {
		return Start{}, fmt.Errorf("%w: build an authentication request for %s: %w",
			ErrUnavailable, found.EntityID, err)
	}

	sealed, err := p.seal(pending{
		RequestID: request.ID,
		ReturnTo:  safe,
		ExpiresAt: p.now().Add(p.stateTTL),
	})
	if err != nil {
		return Start{}, err
	}

	// The request ID travels as RelayState as well, and is *never read back*:
	// the profile caps RelayState at 80 bytes, which is far too small to seal,
	// so it cannot be trusted and nothing here trusts it. It is sent because
	// some identity providers log it and because an operator comparing a
	// provider's log with ours has something to match on.
	redirect, err := request.Redirect(request.ID, &sp)
	if err != nil {
		return Start{}, fmt.Errorf("%w: encode the authentication request for %s: %w",
			ErrUnavailable, found.EntityID, err)
	}

	return Start{RedirectURL: redirect.String(), Sealed: sealed}, nil
}

// Callback is what arrived at the assertion consumer: the two form fields of the
// HTTP-POST binding, and the sealed pending request out of the cookie.
type Callback struct {
	// Response is the base64-encoded `<samlp:Response>`.
	Response string

	// RelayState is what the provider echoed back. It is carried so that it can
	// be logged and is deliberately not used for anything: see [Provider.Start].
	RelayState string

	// Sealed is the cookie's value, from [SealedFrom]. Empty for an
	// identity-provider-initiated sign-in, which started in no browser here.
	Sealed string
}

// Identity is what the identity provider vouches for, verified. It is attributes
// and nothing else: no account exists yet as far as this package is concerned,
// and whether one should is internal/authn's decision.
type Identity struct {
	// Subject is the assertion's `NameID`: the provider's identifier for this
	// person, and what an account is linked by. An address is not, for the
	// reason internal/authn/oidc gives at length — a reassigned address is an
	// account takeover.
	Subject string

	// Email is the address the assertion carried, and DisplayName is the best
	// name it offered, falling back to the address.
	Email       string
	DisplayName string

	// Groups are the values of the configured group attributes, and are empty
	// when the assertion carried none.
	Groups []string

	// ReturnTo is the path this sign-in should land on, already checked. It is
	// empty for an identity-provider-initiated sign-in, which named none.
	ReturnTo string

	// IDPInitiated records that this assertion answered no request from here,
	// so that the log says which of the two kinds of sign-in happened. It is
	// the one place the weaker binding is visible after the fact.
	IDPInitiated bool
}

// Complete finishes a sign-in: it validates the assertion, refuses one that has
// been seen before, and reads the attributes out of it.
//
// The order is the order below, and it is the security of this package:
//
//  1. The pending cookie, which decides *which kind* of sign-in this is, and on
//     a deployment that does not accept identity-provider-initiated ones is
//     also the check that this assertion answers something this browser started.
//  2. The library: signature, issuer, audience, recipient, destination,
//     conditions, and — for a service-provider-initiated sign-in — the
//     `InResponseTo` binding to the request ID out of that cookie.
//  3. This deployment's configured clock skew, which is narrower than the
//     ceiling the library was given.
//  4. Replay. Last, and deliberately: an assertion that has not been proved
//     genuine must not be able to write anything, or an attacker could fill the
//     cache with IDs they chose and lock out the real ones.
func (p *Provider) Complete(ctx context.Context, in Callback) (Identity, error) {
	found, err := p.ready(ctx)
	if err != nil {
		return Identity{}, err
	}

	decoded, err := decodeResponse(in.Response)
	if err != nil {
		return Identity{}, err
	}

	state, initiated, err := p.pendingFor(in.Sealed)
	if err != nil {
		return Identity{}, err
	}

	consumer, possible := p.consumer(found, state, initiated)
	assertion, err := consumer.ParseXMLResponse(decoded, possible, consumer.AcsURL)
	if err != nil {
		// The library's Error() is deliberately a static string and its
		// PrivateErr carries the reason. Unwrapped here so the log says which
		// check failed; the response says none of it.
		return Identity{}, fmt.Errorf("%w: %s", ErrRejected, reasonOf(err))
	}

	now := p.now()
	if err := p.checkFreshness(assertion, now); err != nil {
		return Identity{}, err
	}
	if err := p.assertions.Consume(ctx, assertion.ID, p.replayableUntil(assertion, now)); err != nil {
		// Any failure to record it is a refusal. A cache that is unavailable is
		// a cache that cannot say "no", and accepting an assertion it could not
		// be asked about is the one direction that is not safe to fail in.
		p.log.WarnContext(ctx, "refused a SAML assertion the replay cache would not accept",
			slog.String("assertion_id", assertion.ID),
			slog.String("error", err.Error()))
		return Identity{}, fmt.Errorf("%w: %w", ErrReplayed, err)
	}

	identity, err := p.identityOf(assertion)
	if err != nil {
		return Identity{}, err
	}
	identity.IDPInitiated = initiated
	if identity.ReturnTo, err = returnto.Safe(state.ReturnTo); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// pendingFor opens the sealed cookie and reports whether this is an
// identity-provider-initiated sign-in.
//
// No cookie is the IdP-initiated case, and it is the only one: a cookie that is
// present and will not open is refused rather than quietly demoted to
// IdP-initiated, because otherwise every deployment that accepts IdP-initiated
// sign-in would have the browser binding removable by anybody who can make a
// browser send a corrupt cookie.
func (p *Provider) pendingFor(sealed string) (pending, bool, error) {
	if sealed != "" {
		state, err := p.open(sealed)
		return state, false, err
	}
	if !p.idpInitiated {
		return pending{}, false, fmt.Errorf("%w: the assertion carries no pending request cookie and "+
			"this deployment does not accept identity-provider-initiated sign-in", ErrNoPendingSignIn)
	}
	return pending{}, true, nil
}

// consumer returns the service provider to validate this assertion with, and the
// request IDs it may answer.
//
// It is a *copy* per assertion, and that is the point. `AllowIDPInitiated` on
// github.com/crewjam/saml turns off the `InResponseTo` checks entirely — both
// the response's and the subject confirmation's — so a provider configured to
// accept portal sign-ins would lose the browser binding on the ones that started
// here too. Deciding it per assertion keeps the strong check for the sign-ins
// that can have it, and gives it up only for the ones that structurally cannot.
func (p *Provider) consumer(metadata *crewjam.EntityDescriptor, state pending,
	idpInitiated bool) (crewjam.ServiceProvider, []string) {
	sp := p.template
	sp.IDPMetadata = metadata
	sp.AllowIDPInitiated = idpInitiated

	if idpInitiated {
		return sp, nil
	}
	return sp, []string{state.RequestID}
}

// checkFreshness enforces this deployment's configured clock skew, which is
// narrower than the ceiling the library was given at init. See [maxAllowedSkew]
// for why the two are separate.
//
// It re-checks the windows the library already checked, at the tolerance the
// operator actually asked for. Every failure is [ErrRejected] with the reason in
// the message, for the log.
func (p *Provider) checkFreshness(assertion *crewjam.Assertion, now time.Time) error {
	if assertion.Conditions != nil {
		conditions := assertion.Conditions
		if !conditions.NotBefore.IsZero() && now.Before(conditions.NotBefore.Add(-p.skew)) {
			return fmt.Errorf("%w: it is not valid before %s and it is %s",
				ErrRejected, conditions.NotBefore.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		if !conditions.NotOnOrAfter.IsZero() && !now.Before(conditions.NotOnOrAfter.Add(p.skew)) {
			return fmt.Errorf("%w: it expired at %s and it is %s",
				ErrRejected, conditions.NotOnOrAfter.Format(time.RFC3339), now.Format(time.RFC3339))
		}
	}
	if assertion.Subject != nil {
		for _, confirmation := range assertion.Subject.SubjectConfirmations {
			data := confirmation.SubjectConfirmationData
			if data == nil {
				continue
			}
			if !data.NotBefore.IsZero() && now.Before(data.NotBefore.Add(-p.skew)) {
				return fmt.Errorf("%w: its subject confirmation is not valid before %s and it is %s",
					ErrRejected, data.NotBefore.Format(time.RFC3339), now.Format(time.RFC3339))
			}
			if !data.NotOnOrAfter.IsZero() && !now.Before(data.NotOnOrAfter.Add(p.skew)) {
				return fmt.Errorf("%w: its subject confirmation expired at %s and it is %s",
					ErrRejected, data.NotOnOrAfter.Format(time.RFC3339), now.Format(time.RFC3339))
			}
		}
	}
	if !assertion.IssueInstant.IsZero() &&
		now.After(assertion.IssueInstant.Add(defaultIssueDelay+p.skew)) {
		return fmt.Errorf("%w: it was issued at %s, which is more than %s ago",
			ErrRejected, assertion.IssueInstant.Format(time.RFC3339), defaultIssueDelay+p.skew)
	}
	return nil
}

// replayableUntil is the last moment this assertion could still be presented and
// accepted, and so how long the replay cache has to remember it.
//
// The latest window in the document, widened by the configured skew. Taking the
// latest rather than the earliest is the safe direction: an entry kept too long
// refuses a replay that would have failed anyway, and one dropped too early
// accepts a replay that would have worked.
func (p *Provider) replayableUntil(assertion *crewjam.Assertion, now time.Time) time.Time {
	latest := now.Add(defaultIssueDelay)
	consider := func(t time.Time) {
		if !t.IsZero() && t.After(latest) {
			latest = t
		}
	}
	if assertion.Conditions != nil {
		consider(assertion.Conditions.NotOnOrAfter)
	}
	if assertion.Subject != nil {
		for _, confirmation := range assertion.Subject.SubjectConfirmations {
			if data := confirmation.SubjectConfirmationData; data != nil {
				consider(data.NotOnOrAfter)
			}
		}
	}
	return latest.Add(p.skew)
}

// decodeResponse turns the form field into the XML the library parses.
//
// Whitespace is stripped first: the value is base64 in a form field, and more
// than one identity provider wraps it at 76 columns because that is what their
// XML tooling does.
func decodeResponse(raw string) ([]byte, error) {
	compact := strings.Join(strings.Fields(raw), "")
	if compact == "" {
		return nil, fmt.Errorf("%w: the assertion consumer was posted an empty SAMLResponse", ErrRejected)
	}
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("%w: the SAMLResponse is not base64: %w", ErrRejected, err)
	}
	return decoded, nil
}

// reasonOf digs the real error out of the library's deliberately opaque one.
//
// github.com/crewjam/saml returns an *InvalidResponseError whose Error() is a
// static string, so that an application which prints it does not hand an
// attacker the result of each check. The real reason is in PrivateErr, and this
// deployment does want it — in the log, which is where the whole of this
// package's diagnostics live.
func reasonOf(err error) string {
	var invalid *crewjam.InvalidResponseError
	if errors.As(err, &invalid) && invalid.PrivateErr != nil {
		return invalid.PrivateErr.Error()
	}
	return err.Error()
}
