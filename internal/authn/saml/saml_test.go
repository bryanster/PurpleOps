package saml

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/saml/samltest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
)

// The SAML protocol, against a real identity provider that signs with a real
// key (M1-010). internal/httpapi has the tests about what a verified assertion
// becomes — which account, which cookies, which redirect; these are about which
// documents are accepted at all.
//
// Every rejection below is one field of samltest.Assertion away from the
// happy path above it, which is the point: each test differs from a working
// sign-in by exactly the thing it is about.

const (
	testACSURL      = "https://blacklight.example.com/api/v1/auth/saml/acs"
	testEntityID    = "https://blacklight.example.com/api/v1/auth/saml/metadata"
	testMetadataURL = testEntityID
	testCookiePath  = "/api/v1/auth/saml"
)

// testSealKey is 32 bytes of nothing in particular. It is the deployment
// encryption key, and the sealed cookie is the only thing in these tests derived
// from it.
var testSealKey = []byte("0123456789abcdef0123456789abcdef")

// person is who the test identity provider vouches for throughout.
var person = samltest.Person{
	NameID:      "rowan-persistent-id",
	Email:       "rowan@example.com",
	DisplayName: "Rowan Ash",
	Groups:      []string{"blacklight-admins", "everyone"},
}

// memoryAssertions is the replay cache, in a map. The database-backed one is
// tested in internal/store/identity, and end to end in internal/httpapi against
// a real DuckDB — what these tests need is the *rule*, which is that Consume
// fails the second time.
type memoryAssertions struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newMemoryAssertions() *memoryAssertions {
	return &memoryAssertions{seen: map[string]time.Time{}}
}

func (m *memoryAssertions) Consume(_ context.Context, id string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, used := m.seen[id]; used {
		return errors.New("that assertion has already been used")
	}
	m.seen[id] = expiresAt
	return nil
}

// harness is a service provider pointed at a test identity provider.
type harness struct {
	*Provider
	idp   *samltest.Provider
	cache *memoryAssertions
}

// newHarness builds one. adjust changes the options for the tests that are about
// one of the settings.
func newHarness(t *testing.T, adjust ...func(*Options)) *harness {
	t.Helper()

	idp := samltest.New(t)
	cache := newMemoryAssertions()

	// The service provider's key pair goes through the PEM files and the loaders
	// a deployment uses, rather than being handed over in memory: a test that
	// skipped that would not notice the day the loaders stopped working.
	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())
	certificate, err := loadCertificate(certFile)
	if err != nil {
		t.Fatalf("loading the test certificate: %v", err)
	}
	key, err := loadKey(keyFile)
	if err != nil {
		t.Fatalf("loading the test key: %v", err)
	}

	opts := Options{
		MetadataURL:       idp.MetadataURL(),
		EntityID:          testEntityID,
		ACSURL:            testACSURL,
		OurMetadataURL:    testMetadataURL,
		Certificate:       certificate,
		SigningKey:        key,
		EmailAttributes:   []string{"email", "mail"},
		NameAttributes:    []string{"displayName", "name"},
		GroupAttributes:   []string{"groups", "memberOf"},
		AutoProvision:     true,
		AllowIDPInitiated: false,
		ClockSkew:         2 * time.Minute,
		Assertions:        cache,
		SealKey:           testSealKey,
		CookiePath:        testCookiePath,
		HTTPClient:        idp.Client(),
		Log:               quietLogger(t),
	}
	if err := parseRoleMap(&opts, "blacklight-admins=admin,everyone=member"); err != nil {
		t.Fatalf("parsing the test role map: %v", err)
	}
	for _, fn := range adjust {
		fn(&opts)
	}

	provider, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &harness{Provider: provider, idp: idp, cache: cache}
}

func parseRoleMap(opts *Options, raw string) error {
	var roles config.RoleMap
	if err := roles.UnmarshalText([]byte(raw)); err != nil {
		return err
	}
	opts.RoleMap = roles
	return nil
}

// signIn drives a whole service-provider-initiated sign-in and returns what
// Complete made of it.
func (h *harness) signIn(t *testing.T, returnTo string, in samltest.Assertion) (Identity, error) {
	t.Helper()

	start, err := h.Start(t.Context(), returnTo)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	form := h.idp.Login(t, start.RedirectURL, testACSURL, testEntityID, in)

	return h.Complete(t.Context(), Callback{
		Response:   form.Get("SAMLResponse"),
		RelayState: form.Get("RelayState"),
		Sealed:     start.Sealed,
	})
}

// refuses asserts that a sign-in was rejected, and that the rejection is the
// kind a caller is answered 401 for rather than a fault.
func refuses(t *testing.T, err error, because string) {
	t.Helper()

	switch {
	case err == nil:
		t.Fatalf("the assertion was accepted; it should have been refused because %s", because)
	case !errors.Is(err, ErrRejected) && !errors.Is(err, ErrNoPendingSignIn):
		t.Fatalf("the assertion was refused with %v, which is neither ErrRejected nor "+
			"ErrNoPendingSignIn — a caller would be answered a 500 rather than a 401", err)
	}
}

// --- The one that works --------------------------------------------------------

func TestAValidAssertionIsAccepted(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	identity, err := h.signIn(t, "/engagements", samltest.Assertion{Person: person})
	if err != nil {
		t.Fatalf("a correct assertion was refused: %v", err)
	}

	if identity.Subject != person.NameID {
		t.Errorf("subject = %q, want %q", identity.Subject, person.NameID)
	}
	if identity.Email != person.Email {
		t.Errorf("email = %q, want %q", identity.Email, person.Email)
	}
	if identity.DisplayName != person.DisplayName {
		t.Errorf("displayName = %q, want %q", identity.DisplayName, person.DisplayName)
	}
	if identity.ReturnTo != "/engagements" {
		t.Errorf("returnTo = %q, want %q", identity.ReturnTo, "/engagements")
	}
	if identity.IDPInitiated {
		t.Error("a sign-in that started here is reported as identity-provider-initiated")
	}
}

func TestAResponseSignedInsteadOfTheAssertionIsAccepted(t *testing.T) {
	t.Parallel()

	// Both are valid per the profile — a provider may sign the response, the
	// assertion, or both — and a service provider that only accepted one of them
	// would be broken against half the products on the market.
	h := newHarness(t)

	if _, err := h.signIn(t, "", samltest.Assertion{
		Person:       person,
		SignResponse: true,
	}); err != nil {
		t.Fatalf("an assertion inside a signed response was refused: %v", err)
	}
}

// --- The rejections ------------------------------------------------------------

// TestAnUnsignedAssertionIsRejected. Nothing in an unsigned assertion is
// evidence of anything: it is a text file naming whoever wrote it.
func TestAnUnsignedAssertionIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{Person: person, Unsigned: true})
	refuses(t, err, "nothing signed it")
}

// TestAnAssertionSignedByTheWrongKeyIsRejected. The signature is perfect; the
// key is not the one the identity provider publishes. This is the attack of
// anybody who can obtain a certificate, which is anybody.
func TestAnAssertionSignedByTheWrongKeyIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{Person: person, WrongKey: true})
	refuses(t, err, "it is signed by a key the identity provider does not publish")
}

// TestATamperedAssertionIsRejected — the document was edited after it was
// signed. This is the check that a canonicalizing implementation gives and a
// hand-rolled one does not.
func TestATamperedAssertionIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{Person: person, Tamper: true})
	refuses(t, err, "an attribute was edited after signing")
}

// TestAnExpiredAssertionIsRejected. Expiry is what bounds every other guarantee
// in the protocol; without it a captured assertion is a permanent credential.
func TestAnExpiredAssertionIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	past := time.Now().UTC().Add(-time.Hour)

	_, err := h.signIn(t, "", samltest.Assertion{
		Person:       person,
		IssueInstant: past,
		NotBefore:    past.Add(-time.Minute),
		NotOnOrAfter: past.Add(time.Minute),
	})
	refuses(t, err, "it expired an hour ago")
}

// TestAnAssertionOutsideItsNotBeforeWindowIsRejected — one minted for later,
// which is what an identity provider with a fast clock produces and what
// somebody who captured a pre-issued assertion would present early.
func TestAnAssertionOutsideItsNotBeforeWindowIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	future := time.Now().UTC().Add(time.Hour)

	_, err := h.signIn(t, "", samltest.Assertion{
		Person:       person,
		NotBefore:    future,
		NotOnOrAfter: future.Add(assertionWindow),
	})
	refuses(t, err, "it is not valid until an hour from now")
}

// TestAReplayedAssertionIsRejected is the one no library does for you. The
// assertion is entirely valid — it is the same one, twice.
func TestAReplayedAssertionIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	start, err := h.Start(t.Context(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	form := h.idp.Login(t, start.RedirectURL, testACSURL, testEntityID,
		samltest.Assertion{Person: person})

	callback := Callback{
		Response:   form.Get("SAMLResponse"),
		RelayState: form.Get("RelayState"),
		Sealed:     start.Sealed,
	}
	if _, err := h.Complete(t.Context(), callback); err != nil {
		t.Fatalf("the first presentation was refused: %v", err)
	}

	// Byte for byte the same document, the same cookie, inside the same validity
	// window. Everything the protocol checks still passes.
	_, err = h.Complete(t.Context(), callback)
	refuses(t, err, "it has been presented before")
	if !errors.Is(err, ErrReplayed) {
		t.Errorf("the second presentation failed with %v, want ErrReplayed — the replay cache is "+
			"not what refused it, so something else did and this test proves nothing", err)
	}
}

// TestAnAssertionForAnotherAudienceIsRejected. An assertion valid at a
// *different* service provider, presented here — which is what an attacker who
// also uses your identity provider has, for free, every day.
func TestAnAssertionForAnotherAudienceIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{
		Person:   person,
		Audience: "https://someone-elses-app.example.com/saml/metadata",
	})
	refuses(t, err, "it is audience-restricted to another service provider")
}

// TestAnAssertionForAnotherRecipientIsRejected — the same idea one level down:
// the subject confirmation names an assertion consumer that is not this one.
func TestAnAssertionForAnotherRecipientIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{
		Person:    person,
		Recipient: "https://someone-elses-app.example.com/saml/acs",
	})
	refuses(t, err, "its Recipient is another assertion consumer")
}

// TestAnAssertionSentToAnotherDestinationIsRejected — and one level up: the
// response says it was addressed somewhere else.
func TestAnAssertionSentToAnotherDestinationIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.signIn(t, "", samltest.Assertion{
		Person:      person,
		Destination: "https://someone-elses-app.example.com/saml/acs",
	})
	refuses(t, err, "its Destination is another assertion consumer")
}

// TestAnAssertionAnsweringAnotherRequestIsRejected is login CSRF: an attacker
// signs in at the identity provider, keeps the assertion minted for *their*
// account, and delivers it into your browser. Everything about it is valid
// except that it does not answer the request your browser was handed.
func TestAnAssertionAnsweringAnotherRequestIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	// The attacker's sign-in, and the assertion it produced.
	theirs, err := h.Start(t.Context(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	form := h.idp.Login(t, theirs.RedirectURL, testACSURL, testEntityID,
		samltest.Assertion{Person: samltest.Person{
			NameID: "mallory-persistent-id", Email: "mallory@example.com",
		}})

	// Your browser, which started a sign-in of its own.
	yours, err := h.Start(t.Context(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	_, err = h.Complete(t.Context(), Callback{
		Response: form.Get("SAMLResponse"),
		Sealed:   yours.Sealed,
	})
	refuses(t, err, "it answers a request this browser was never handed")
}

// TestAnAssertionWithNoPendingRequestIsRejectedWhenPortalSignInIsOff. With
// identity-provider-initiated sign-in turned off there is no such thing as an
// assertion that answers nothing.
func TestAnAssertionWithNoPendingRequestIsRejectedWhenPortalSignInIsOff(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.Complete(t.Context(), Callback{
		Response: h.idp.Respond(t, testACSURL, testEntityID, samltest.Assertion{Person: person}),
	})
	refuses(t, err, "it answers no request and this deployment does not accept portal sign-in")
	if !errors.Is(err, ErrNoPendingSignIn) {
		t.Errorf("err = %v, want ErrNoPendingSignIn", err)
	}
}

// TestACorruptPendingCookieIsRejectedRatherThanIgnored. The dangerous version of
// the case above: on a deployment that *does* accept portal sign-in, a cookie
// that will not open must not be treated as no cookie at all — that would make
// the browser binding removable by anybody who can corrupt one.
func TestACorruptPendingCookieIsRejectedRatherThanIgnored(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *Options) { o.AllowIDPInitiated = true })

	_, err := h.Complete(t.Context(), Callback{
		Response: h.idp.Respond(t, testACSURL, testEntityID, samltest.Assertion{Person: person}),
		Sealed:   "not-a-cookie-this-deployment-sealed",
	})
	refuses(t, err, "the cookie it carries will not open")
	if !errors.Is(err, ErrNoPendingSignIn) {
		t.Errorf("err = %v, want ErrNoPendingSignIn", err)
	}
}

// TestAnExpiredPendingCookieIsRejected: a sign-in left open in a browser is not
// still completable tomorrow.
func TestAnExpiredPendingCookieIsRejected(t *testing.T) {
	t.Parallel()

	// A sealed cookie that was already stale when it was minted, by moving the
	// provider's own clock back before it seals and forward again afterwards.
	h := newHarness(t, func(o *Options) { o.StateTTL = time.Millisecond })

	start, err := h.Start(t.Context(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	form := h.idp.Login(t, start.RedirectURL, testACSURL, testEntityID,
		samltest.Assertion{Person: person})
	time.Sleep(5 * time.Millisecond)

	_, err = h.Complete(t.Context(), Callback{
		Response: form.Get("SAMLResponse"),
		Sealed:   start.Sealed,
	})
	refuses(t, err, "the sign-in it belongs to expired")
}

// --- Identity-provider-initiated sign-in ----------------------------------------

// TestAPortalSignInIsAcceptedWhenItIsAllowed, and is marked as what it is. The
// weaker binding is a configuration choice, and the [Identity] says which kind
// of sign-in happened so that a log can too.
func TestAPortalSignInIsAcceptedWhenItIsAllowed(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *Options) { o.AllowIDPInitiated = true })

	identity, err := h.Complete(t.Context(), Callback{
		Response: h.idp.Respond(t, testACSURL, testEntityID, samltest.Assertion{Person: person}),
	})
	if err != nil {
		t.Fatalf("a portal sign-in was refused on a deployment that allows them: %v", err)
	}
	if !identity.IDPInitiated {
		t.Error("the identity does not record that this sign-in answered no request from here")
	}
	if identity.ReturnTo != "" {
		t.Errorf("returnTo = %q, want empty: a portal sign-in started on no page here", identity.ReturnTo)
	}
}

// TestAPortalSignInIsStillSubjectToEveryOtherCheck. Allowing them gives up the
// browser binding and nothing else — which is worth asserting, because the
// library's own switch for this gives up more.
func TestAPortalSignInIsStillSubjectToEveryOtherCheck(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *Options) { o.AllowIDPInitiated = true })

	for name, assertion := range map[string]samltest.Assertion{
		"unsigned":       {Person: person, Unsigned: true},
		"wrong key":      {Person: person, WrongKey: true},
		"tampered":       {Person: person, Tamper: true},
		"wrong audience": {Person: person, Audience: "https://elsewhere.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.Complete(t.Context(), Callback{
				Response: h.idp.Respond(t, testACSURL, testEntityID, assertion),
			})
			refuses(t, err, name)
		})
	}
}

// TestAPortalSignInIsAlsoRefusedTwice — the replay cache does not care which
// kind of sign-in produced the assertion.
func TestAPortalSignInIsAlsoRefusedTwice(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *Options) { o.AllowIDPInitiated = true })
	response := h.idp.Respond(t, testACSURL, testEntityID, samltest.Assertion{Person: person})

	if _, err := h.Complete(t.Context(), Callback{Response: response}); err != nil {
		t.Fatalf("the first presentation was refused: %v", err)
	}
	_, err := h.Complete(t.Context(), Callback{Response: response})
	refuses(t, err, "it has been presented before")
}

// --- Clock skew ------------------------------------------------------------------

// assertionWindow is how long the test assertions are valid for, matching what
// samltest mints by default.
const assertionWindow = 5 * time.Minute

// TestClockSkewIsBoundedAndConfigurable. The library keeps its tolerance in a
// package-level variable with a three-minute default; this deployment's is the
// configured one, enforced afterwards, and it is what actually decides.
func TestClockSkewIsBoundedAndConfigurable(t *testing.T) {
	t.Parallel()

	// An assertion that expired ninety seconds ago. Inside a two-minute skew,
	// outside a zero one.
	expiredRecently := func() samltest.Assertion {
		at := time.Now().UTC().Add(-90 * time.Second)
		return samltest.Assertion{
			Person:       person,
			IssueInstant: at.Add(-time.Minute),
			NotBefore:    at.Add(-2 * time.Minute),
			NotOnOrAfter: at,
		}
	}

	t.Run("inside the configured skew it is accepted", func(t *testing.T) {
		h := newHarness(t, func(o *Options) { o.ClockSkew = 2 * time.Minute })
		if _, err := h.signIn(t, "", expiredRecently()); err != nil {
			t.Fatalf("an assertion 90s past its expiry was refused with a 2m skew: %v", err)
		}
	})

	t.Run("outside it, it is not", func(t *testing.T) {
		h := newHarness(t, func(o *Options) { o.ClockSkew = 0 })
		_, err := h.signIn(t, "", expiredRecently())
		refuses(t, err, "it expired 90 seconds ago and this deployment allows no skew")
	})
}

// TestASkewWiderThanTheCeilingIsRefusedAtConstruction. The ceiling is what the
// library was given at init; a provider configured beyond it would be one whose
// configured tolerance the library silently overrode.
func TestASkewWiderThanTheCeilingIsRefusedAtConstruction(t *testing.T) {
	t.Parallel()

	idp := samltest.New(t)
	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())
	certificate, err := loadCertificate(certFile)
	if err != nil {
		t.Fatalf("loading the certificate: %v", err)
	}
	key, err := loadKey(keyFile)
	if err != nil {
		t.Fatalf("loading the key: %v", err)
	}

	_, err = New(Options{
		MetadataURL:     idp.MetadataURL(),
		EntityID:        testEntityID,
		ACSURL:          testACSURL,
		OurMetadataURL:  testMetadataURL,
		Certificate:     certificate,
		SigningKey:      key,
		EmailAttributes: []string{"email"},
		ClockSkew:       maxAllowedSkew + time.Second,
		Assertions:      newMemoryAssertions(),
		SealKey:         testSealKey,
		CookiePath:      testCookiePath,
	})
	if err == nil {
		t.Fatal("a clock skew beyond the ceiling was accepted")
	}
}

// --- Construction ------------------------------------------------------------------

// TestAProviderWithNoReplayCacheIsRefused. It is the one misconfiguration in
// this package that looks exactly like success: every assertion is accepted, as
// many times as it is presented.
func TestAProviderWithNoReplayCacheIsRefused(t *testing.T) {
	t.Parallel()

	idp := samltest.New(t)
	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())
	certificate, _ := loadCertificate(certFile) //nolint:errcheck // covered by newHarness
	key, _ := loadKey(keyFile)                  //nolint:errcheck // covered by newHarness

	_, err := New(Options{
		MetadataURL:     idp.MetadataURL(),
		EntityID:        testEntityID,
		ACSURL:          testACSURL,
		OurMetadataURL:  testMetadataURL,
		Certificate:     certificate,
		SigningKey:      key,
		EmailAttributes: []string{"email"},
		SealKey:         testSealKey,
		CookiePath:      testCookiePath,
	})
	if err == nil {
		t.Fatal("a provider with no replay cache was built")
	}
	if !strings.Contains(err.Error(), "replay") {
		t.Errorf("err = %v, want it to name the replay cache", err)
	}
}

// --- Metadata ------------------------------------------------------------------

// TestTheMetadataCarriesTheCertificateAndNotTheKey is the acceptance criterion
// that the service provider's private key is never exposed by any endpoint.
func TestTheMetadataCarriesTheCertificateAndNotTheKey(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	document, err := h.Metadata()
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	text := string(document)

	descriptor, err := parseOwnMetadata(document)
	if err != nil {
		t.Fatalf("the metadata this server publishes does not parse as metadata: %v\n%s", err, text)
	}
	if descriptor.EntityID != testEntityID {
		t.Errorf("entityID = %q, want %q", descriptor.EntityID, testEntityID)
	}
	if len(descriptor.SPSSODescriptors) != 1 {
		t.Fatalf("the document declares %d service provider descriptors, want 1",
			len(descriptor.SPSSODescriptors))
	}
	acs := descriptor.SPSSODescriptors[0].AssertionConsumerServices
	if len(acs) != 1 || acs[0].Location != testACSURL {
		t.Errorf("assertion consumer services = %+v, want exactly one at %s", acs, testACSURL)
	}

	// The private key, in every spelling it could plausibly leak in.
	for _, forbidden := range []string{
		"PRIVATE KEY", "RSA PRIVATE KEY", "BEGIN PRIVATE", "privateKey",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("the published metadata contains %q:\n%s", forbidden, text)
		}
	}

	// Read off the parsed document rather than the text, so this asserts the
	// certificate is somewhere an identity provider will look for it.
	var signing int
	for _, key := range descriptor.SPSSODescriptors[0].KeyDescriptors {
		if key.Use == "signing" && len(key.KeyInfo.X509Data.X509Certificates) > 0 {
			signing++
		}
	}
	if signing == 0 {
		t.Errorf("the published metadata carries no signing certificate, so no identity provider "+
			"could check a signature from this deployment:\n%s", text)
	}
}

// TestTheMetadataAdvertisesOnlyTheBindingWeImplement. Advertising the artifact
// binding would let an identity provider choose one the assertion consumer does
// not accept, and the failure would be a sign-in that never reaches any code
// written here.
func TestTheMetadataAdvertisesOnlyTheBindingWeImplement(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	document, err := h.Metadata()
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if strings.Contains(string(document), "HTTP-Artifact") {
		t.Errorf("the published metadata advertises the artifact binding:\n%s", document)
	}
}

// TestMetadataIsServedWhileTheIdentityProviderIsDown. It is the document
// somebody fetches while setting the registration up, which is exactly when the
// other half does not work yet.
func TestMetadataIsServedWhileTheIdentityProviderIsDown(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.idp.Close()

	if _, err := h.Metadata(); err != nil {
		t.Fatalf("Metadata with the identity provider down: %v", err)
	}
}

// TestAnUnreachableIdentityProviderIsUnavailableRatherThanBroken. PLAN.md §4:
// a broken identity provider must never lock anybody out. The button is absent
// and local sign-in is untouched.
func TestAnUnreachableIdentityProviderIsUnavailableRatherThanBroken(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.idp.Close()

	if h.Available(t.Context()) {
		t.Error("Available() is true with the identity provider stopped")
	}

	_, err := h.Start(t.Context(), "")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("Start with the provider down = %v, want ErrUnavailable", err)
	}
}

// TestMetadataFromAFileNeedsNoNetworkAtAll — the configuration for a provider
// that publishes XML and no URL.
func TestMetadataFromAFileNeedsNoNetworkAtAll(t *testing.T) {
	t.Parallel()

	idp := samltest.New(t)
	document := idp.Metadata(t)
	idp.Close()

	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())
	certificate, err := loadCertificate(certFile)
	if err != nil {
		t.Fatalf("loading the certificate: %v", err)
	}
	key, err := loadKey(keyFile)
	if err != nil {
		t.Fatalf("loading the key: %v", err)
	}

	provider, err := New(Options{
		MetadataXML:     document,
		EntityID:        testEntityID,
		ACSURL:          testACSURL,
		OurMetadataURL:  testMetadataURL,
		Certificate:     certificate,
		SigningKey:      key,
		EmailAttributes: []string{"email"},
		ClockSkew:       time.Minute,
		Assertions:      newMemoryAssertions(),
		SealKey:         testSealKey,
		CookiePath:      testCookiePath,
		Log:             quietLogger(t),
	})
	if err != nil {
		t.Fatalf("New with static metadata: %v", err)
	}
	if !provider.Available(t.Context()) {
		t.Error("a provider configured from a file is unavailable, with nothing to fetch")
	}
}

// TestMetadataWithNoSigningCertificateIsRefused, at configuration time rather
// than at somebody's first login.
func TestMetadataWithNoSigningCertificateIsRefused(t *testing.T) {
	t.Parallel()

	const keyless = `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
      Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	if _, err := parseMetadata([]byte(keyless)); err == nil {
		t.Fatal("metadata with no signing certificate was accepted; every assertion under it " +
			"would fail to verify, and finding that out at a first login is finding it out too late")
	}
}

// --- Role mapping ------------------------------------------------------------------

// TestGroupsMapToPlatformRoles, through the same [config.RoleMap] the OIDC path
// uses — which is the reuse M1-010 asks for rather than a second mapping.
func TestGroupsMapToPlatformRoles(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	tests := map[string]struct {
		groups []string
		want   authz.PlatformRole
		mapped bool
	}{
		"the strongest match wins": {
			groups: []string{"everyone", "blacklight-admins"},
			want:   authz.PlatformRoleAdmin, mapped: true,
		},
		"order at the provider does not decide": {
			groups: []string{"blacklight-admins", "everyone"},
			want:   authz.PlatformRoleAdmin, mapped: true,
		},
		"a mapped group maps": {
			groups: []string{"everyone"},
			want:   authz.PlatformRoleMember, mapped: true,
		},
		"an unmapped group says nothing at all": {
			groups: []string{"some-other-directory-group"},
			mapped: false,
		},
		"no groups says nothing at all": {mapped: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			role, mapped := h.Role(tc.groups)
			if mapped != tc.mapped {
				t.Fatalf("mapped = %v, want %v", mapped, tc.mapped)
			}
			if mapped && role != tc.want {
				t.Errorf("role = %q, want %q", role, tc.want)
			}
		})
	}
}

// --- The return path ------------------------------------------------------------------

// TestAnUnsafeReturnPathIsRefused, at the point it arrives — and it is the same
// rule the OIDC path applies, from internal/authn/returnto.
func TestAnUnsafeReturnPathIsRefused(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	for _, unsafe := range []string{
		"https://evil.example.com/",
		"//evil.example.com/path",
		"/legitimate\\but-with-a-backslash",
		"/with-a\nnewline",
	} {
		if _, err := h.Start(t.Context(), unsafe); err == nil {
			t.Errorf("Start(%q) was accepted; it is an open redirect on the login path", unsafe)
		}
	}
}
