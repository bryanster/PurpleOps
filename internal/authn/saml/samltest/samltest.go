// Package samltest is a SAML identity provider for tests: it publishes
// metadata, reads an authentication request, and mints a signed assertion —
// including, on request, every way of minting one that a service provider must
// refuse.
//
// It exists because the rejection cases *are* the ticket (M1-010). "An unsigned
// assertion is rejected" cannot be asserted by a harness that can only produce
// correct ones, and a test that hand-edits a checked-in blob is a test that
// stops meaning anything the moment the format changes. So every document here
// is built and signed the same way a real identity provider builds one, and each
// attack is one field of [Assertion] away from the document beside it.
//
// It replaces the checked-in fixture corpus M1-010 proposed, and the ticket file
// records why: a live provider cannot go stale, cannot expire, and cannot
// describe an attack the code no longer performs.
//
// Nothing here is fit for any purpose but a test. It signs with a key it made up
// on the spot and it will produce, on request, documents whose whole point is to
// be invalid.
package samltest

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/xml"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/beevik/etree"
	crewjam "github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
)

// certificateLife is how long the generated certificates are valid for.
//
// A century, and the reason is the one M1-010 calls out by name: a test suite
// that dies because a fixture certificate expired is a predictable and
// preventable annoyance. goxmldsig checks certificate validity against the
// clock when it verifies, so a short-lived certificate here would be a timer on
// the whole suite.
const certificateLife = 100 * 365 * 24 * time.Hour

// assertionLife is how long a minted assertion is valid for by default. Five
// minutes is what real identity providers use, and it is long enough that no
// test is racing it.
const assertionLife = 5 * time.Minute

// Provider is a running identity provider: an HTTP server publishing metadata,
// and the key pair every assertion it mints is signed with.
type Provider struct {
	// EntityID is what this identity provider calls itself, and what an
	// assertion's Issuer carries.
	EntityID string

	// SSOURL is where a service provider sends a browser to sign in.
	SSOURL string

	server *httptest.Server

	key  *rsa.PrivateKey
	cert *x509.Certificate

	// The second key pair: a valid, well-formed certificate that this provider's
	// metadata does *not* publish. It is what [Assertion.WrongKey] signs with,
	// which is the difference between "the signature is broken" and "the
	// signature is perfect and belongs to somebody else" — the second being the
	// one an attacker actually has.
	otherKey  *rsa.PrivateKey
	otherCert *x509.Certificate
}

// New starts an identity provider and stops it when the test ends.
func New(t *testing.T) *Provider {
	t.Helper()

	key, cert := keyPair(t, "samltest identity provider")
	otherKey, otherCert := keyPair(t, "samltest impostor")

	p := &Provider{
		key:       key,
		cert:      cert,
		otherKey:  otherKey,
		otherCert: otherCert,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metadata", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/samlmetadata+xml")
		if _, err := w.Write(p.Metadata(t)); err != nil {
			t.Errorf("serving the test identity provider metadata: %v", err)
		}
	})
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)

	p.EntityID = p.server.URL + "/metadata"
	p.SSOURL = p.server.URL + "/sso"
	return p
}

// MetadataURL is where this provider publishes its metadata.
func (p *Provider) MetadataURL() string { return p.server.URL + "/metadata" }

// Client is an HTTP client that can reach it.
func (p *Provider) Client() *http.Client { return p.server.Client() }

// Close stops the server early, for the tests that want a provider which is
// already down. It is safe to call twice; the cleanup registered by [New] does
// the same thing.
func (p *Provider) Close() { p.server.Close() }

// Metadata is the document a service provider is configured from.
func (p *Provider) Metadata(t *testing.T) []byte {
	t.Helper()

	descriptor := crewjam.EntityDescriptor{
		EntityID: p.server.URL + "/metadata",
		IDPSSODescriptors: []crewjam.IDPSSODescriptor{{
			SSODescriptor: crewjam.SSODescriptor{
				RoleDescriptor: crewjam.RoleDescriptor{
					ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
					KeyDescriptors: []crewjam.KeyDescriptor{{
						Use: "signing",
						KeyInfo: crewjam.KeyInfo{
							X509Data: crewjam.X509Data{
								X509Certificates: []crewjam.X509Certificate{{
									Data: base64.StdEncoding.EncodeToString(p.cert.Raw),
								}},
							},
						},
					}},
				},
				NameIDFormats: []crewjam.NameIDFormat{crewjam.PersistentNameIDFormat},
			},
			SingleSignOnServices: []crewjam.Endpoint{
				{Binding: crewjam.HTTPRedirectBinding, Location: p.server.URL + "/sso"},
				{Binding: crewjam.HTTPPostBinding, Location: p.server.URL + "/sso"},
			},
		}},
	}

	encoded, err := xml.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the test identity provider metadata: %v", err)
	}
	return append([]byte(xml.Header), encoded...)
}

// Person is who this identity provider vouches for.
type Person struct {
	NameID      string
	Email       string
	DisplayName string
	Groups      []string
}

// Assertion describes one document to mint. The zero value plus a [Person] is a
// correct assertion; every other field is either a detail worth varying or an
// attack worth refusing.
type Assertion struct {
	Person

	// InResponseTo is the authentication request this answers. Empty is an
	// identity-provider-initiated sign-in.
	InResponseTo string

	// Audience, Recipient and Destination default to the service provider this
	// is being minted for. Setting one is the "addressed to somebody else"
	// attack: an assertion that is perfectly valid at a different service
	// provider, replayed here.
	Audience    string
	Recipient   string
	Destination string

	// ID is the assertion's own identifier. Empty means a fresh one; setting it
	// to a value already used is the replay.
	ID string

	// IssueInstant, NotBefore and NotOnOrAfter default to a window around now.
	// Setting them is how an expired assertion, and one presented before its
	// window opens, are built.
	IssueInstant time.Time
	NotBefore    time.Time
	NotOnOrAfter time.Time

	// Unsigned mints the document with no signature at all — the attack that
	// works against a service provider which parses first and verifies second,
	// or which trusts a `Response` wrapper it never checked.
	Unsigned bool

	// WrongKey signs it correctly with a key the provider's metadata does not
	// publish. The signature verifies against *a* certificate; it is not this
	// identity provider's.
	WrongKey bool

	// Tamper edits an attribute value after signing, leaving the signature
	// covering the document that was signed and the parser reading a different
	// one. This is the whole reason XML signature validation is delegated to a
	// library rather than written here.
	Tamper bool

	// SignResponse signs the enclosing `Response` instead of the assertion. Both
	// are acceptable per the profile and a service provider has to handle
	// either; this is how the "either way" half of that is tested.
	SignResponse bool
}

// Login drives a whole service-provider-initiated sign-in: it reads the
// authentication request out of the redirect URL and answers it.
//
// The returned values are the form the identity provider would POST to the
// assertion consumer.
func (p *Provider) Login(t *testing.T, redirectURL, acsURL, audience string, in Assertion) url.Values {
	t.Helper()

	in.InResponseTo = p.RequestID(t, redirectURL)
	return url.Values{
		"SAMLResponse": {p.Respond(t, acsURL, audience, in)},
		"RelayState":   {in.InResponseTo},
	}
}

// RequestID reads the ID of the authentication request encoded in a
// HTTP-Redirect URL.
func (p *Provider) RequestID(t *testing.T, redirectURL string) string {
	t.Helper()

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("the redirect %q does not parse: %v", redirectURL, err)
	}
	compressed, err := base64.StdEncoding.DecodeString(parsed.Query().Get("SAMLRequest"))
	if err != nil {
		t.Fatalf("the SAMLRequest in %q is not base64: %v", redirectURL, err)
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(compressed)))
	if err != nil {
		t.Fatalf("the SAMLRequest in %q does not inflate: %v", redirectURL, err)
	}

	var request crewjam.AuthnRequest
	if err := xml.Unmarshal(raw, &request); err != nil {
		t.Fatalf("the SAMLRequest in %q is not an AuthnRequest: %v\n%s", redirectURL, err, raw)
	}
	if request.ID == "" {
		t.Fatalf("the AuthnRequest in %q carries no ID:\n%s", redirectURL, raw)
	}
	return request.ID
}

// Respond mints the base64-encoded `<samlp:Response>` a service provider would
// be posted.
func (p *Provider) Respond(t *testing.T, acsURL, audience string, in Assertion) string {
	t.Helper()

	now := time.Now().UTC()
	fill(&in, acsURL, audience, now)

	assertion := p.assertion(in)
	assertionEl := assertion.Element()

	if !in.Unsigned && !in.SignResponse {
		assertionEl = p.sign(t, assertion, in.WrongKey)
	}
	if in.Tamper {
		tamper(t, assertionEl)
	}

	response := &crewjam.Response{
		ID:           "id-response-" + newID(t),
		InResponseTo: in.InResponseTo,
		Version:      "2.0",
		IssueInstant: in.IssueInstant,
		Destination:  in.Destination,
		Issuer: &crewjam.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  p.EntityID,
		},
		Status: crewjam.Status{
			StatusCode: crewjam.StatusCode{Value: crewjam.StatusSuccess},
		},
	}

	responseEl := response.Element()
	// The assertion is attached as an element rather than through
	// Response.Assertion, because the signed form is an *etree.Element and
	// re-marshalling the struct would drop the signature.
	responseEl.AddChild(assertionEl)

	if in.SignResponse {
		responseEl = p.signElement(t, responseEl, in.WrongKey)
	}

	document := etree.NewDocument()
	document.SetRoot(responseEl)
	encoded, err := document.WriteToBytes()
	if err != nil {
		t.Fatalf("serializing the response: %v", err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

// fill applies the defaults that make an [Assertion] a correct one.
func fill(in *Assertion, acsURL, audience string, now time.Time) {
	if in.ID == "" {
		in.ID = "id-assertion-" + randomHex()
	}
	if in.IssueInstant.IsZero() {
		in.IssueInstant = now
	}
	if in.NotBefore.IsZero() {
		in.NotBefore = now.Add(-time.Minute)
	}
	if in.NotOnOrAfter.IsZero() {
		in.NotOnOrAfter = now.Add(assertionLife)
	}
	if in.Audience == "" {
		in.Audience = audience
	}
	if in.Recipient == "" {
		in.Recipient = acsURL
	}
	if in.Destination == "" {
		in.Destination = acsURL
	}
}

// assertion builds the document, before it is signed.
func (p *Provider) assertion(in Assertion) *crewjam.Assertion {
	attributes := []crewjam.Attribute{}
	add := func(name, friendly string, values ...string) {
		if len(values) == 0 {
			return
		}
		attribute := crewjam.Attribute{
			Name:         name,
			FriendlyName: friendly,
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
		}
		for _, value := range values {
			attribute.Values = append(attribute.Values,
				crewjam.AttributeValue{Type: "xs:string", Value: value})
		}
		attributes = append(attributes, attribute)
	}
	if in.Email != "" {
		add("email", "mail", in.Email)
	}
	if in.DisplayName != "" {
		add("displayName", "displayName", in.DisplayName)
	}
	add("groups", "groups", in.Groups...)

	return &crewjam.Assertion{
		ID:           in.ID,
		IssueInstant: in.IssueInstant,
		Version:      "2.0",
		Issuer: crewjam.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  p.EntityID,
		},
		Subject: &crewjam.Subject{
			NameID: &crewjam.NameID{
				Format: string(crewjam.PersistentNameIDFormat),
				Value:  in.NameID,
			},
			SubjectConfirmations: []crewjam.SubjectConfirmation{{
				Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
				SubjectConfirmationData: &crewjam.SubjectConfirmationData{
					InResponseTo: in.InResponseTo,
					NotOnOrAfter: in.NotOnOrAfter,
					Recipient:    in.Recipient,
				},
			}},
		},
		Conditions: &crewjam.Conditions{
			NotBefore:    in.NotBefore,
			NotOnOrAfter: in.NotOnOrAfter,
			AudienceRestrictions: []crewjam.AudienceRestriction{{
				Audience: crewjam.Audience{Value: in.Audience},
			}},
		},
		AuthnStatements: []crewjam.AuthnStatement{{
			AuthnInstant: in.IssueInstant,
			SessionIndex: "session-" + in.ID,
			AuthnContext: crewjam.AuthnContext{
				AuthnContextClassRef: &crewjam.AuthnContextClassRef{
					Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
				},
			},
		}},
		AttributeStatements: []crewjam.AttributeStatement{{Attributes: attributes}},
	}
}

// sign signs the assertion the way an identity provider does: envelope the
// signature, then re-render the element so the struct and the XML agree.
func (p *Provider) sign(t *testing.T, assertion *crewjam.Assertion, wrongKey bool) *etree.Element {
	t.Helper()

	signed := p.signElement(t, assertion.Element(), wrongKey)
	signature, ok := signed.Child[len(signed.Child)-1].(*etree.Element)
	if !ok {
		t.Fatal("the signing context appended something that is not an element")
	}
	assertion.Signature = signature
	return assertion.Element()
}

// signElement is the raw XML-DSig operation, on whichever element is being
// signed.
func (p *Provider) signElement(t *testing.T, element *etree.Element, wrongKey bool) *etree.Element {
	t.Helper()

	key, cert := p.key, p.cert
	if wrongKey {
		key, cert = p.otherKey, p.otherCert
	}

	context := dsig.NewDefaultSigningContext(dsig.TLSCertKeyStore(tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}))
	// The same canonicalizer and algorithm github.com/crewjam/saml's own
	// identity provider uses, so what this mints is what the parser expects.
	context.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	if err := context.SetSignatureMethod(dsig.RSASHA256SignatureMethod); err != nil {
		t.Fatalf("setting the signature method: %v", err)
	}

	signed, err := context.SignEnveloped(element)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	return signed
}

// tamper edits an attribute value after the signature was computed, which is the
// modification a service provider must notice.
//
// The display name, deliberately: it is a field that changes nothing about who
// the assertion is for, so a service provider that accepted this would be one
// that verified *something* and then read something else — which is exactly how
// a signature-wrapping bug looks from the outside.
func tamper(t *testing.T, assertion *etree.Element) {
	t.Helper()

	for _, value := range assertion.FindElements(".//AttributeValue") {
		if value.Text() != "" {
			value.SetText(value.Text() + " (edited after signing)")
			return
		}
	}
	t.Fatal("there was no attribute value to tamper with; the assertion has changed shape")
}

// keyPair mints a self-signed certificate and the key under it.
func keyPair(t *testing.T, name string) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	// 2048 rather than 4096: this runs on every test that needs an identity
	// provider, and the difference is a second of wall clock per run for no
	// property any test asserts.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generating a serial number: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(certificateLife),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	raw, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating a certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatalf("parsing the certificate just created: %v", err)
	}
	return key, certificate
}

// ServiceProviderKeyPair mints a key pair for the *service provider* under test,
// written to two PEM files, because that is how a deployment is configured.
func ServiceProviderKeyPair(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, certificate := keyPair(t, "samltest service provider")
	return writePEM(t, dir, key, certificate)
}

// newID is a fresh identifier for a document.
func newID(t *testing.T) string {
	t.Helper()
	return randomHex()
}

func randomHex() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		// Unreachable on any platform this runs on, and a test helper has
		// nowhere useful to put an error from crypto/rand.
		panic("samltest: crypto/rand: " + err.Error())
	}
	const hex = "0123456789abcdef"
	out := make([]byte, 0, len(raw)*2)
	for _, b := range raw {
		out = append(out, hex[b>>4], hex[b&0x0f])
	}
	return string(out)
}
