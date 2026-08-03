package saml

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	crewjam "github.com/crewjam/saml"
	xrv "github.com/mattermost/xml-roundtrip-validator"
)

// Reading the two documents this registration is built from: the identity
// provider's metadata, which says which key may sign an assertion, and this
// service provider's own key pair, which signs authentication requests.
//
// The identity provider's metadata is the trust anchor of the whole protocol. It
// is fetched lazily and retried, exactly as OIDC discovery is, and for exactly
// the same reason: a provider that is down must be a missing button on the login
// page and never a server that will not start.

// maxMetadataBytes bounds both the fetched and the on-disk document. A metadata
// document is a few kilobytes; a megabyte is already three hundred times more
// than any real one, and the limit is here so that a misconfigured URL pointing
// at something enormous is a bounded read rather than this process's memory.
const maxMetadataBytes = 1 << 20

// ready returns the identity provider's metadata, fetching it first if that has
// not happened yet.
//
// The three states, and why each is worth distinguishing, are the ones
// internal/authn/oidc sets out at length — read, in flight, and neither. The
// difference here is that a deployment configured with a metadata *file* is
// always in the first state: it was parsed in [New] and there is nothing to
// fetch, so none of the rest of this applies to it.
func (p *Provider) ready(ctx context.Context) (*crewjam.EntityDescriptor, error) {
	p.mu.Lock()
	found, inflight, due := p.found, p.inflight, p.fetchDue()
	p.mu.Unlock()

	switch {
	case found != nil:
		return found, nil
	case !inflight && !due:
		return nil, fmt.Errorf("%w: %s was tried too recently to try again",
			ErrUnavailable, p.metadataURL)
	}
	return p.fetchOnce(ctx)
}

// fetchDue reports whether another attempt is allowed. Callers hold p.mu.
func (p *Provider) fetchDue() bool {
	return p.attempted.IsZero() || p.now().Sub(p.attempted) >= p.fetchRetry
}

// cached returns what the last fetch produced, or nil.
func (p *Provider) cached() *crewjam.EntityDescriptor {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.found
}

// fetchOnce reads the metadata, or joins the read already happening.
//
// The attempt is detached from the caller's context and given a deadline of its
// own, for the reason internal/authn/oidc gives: two callers with different
// patience must not be able to cancel each other's work.
func (p *Provider) fetchOnce(ctx context.Context) (*crewjam.EntityDescriptor, error) {
	result := p.fetching.DoChan("fetch", func() (any, error) {
		attempt, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()

		p.mu.Lock()
		p.attempted, p.inflight = p.now(), true
		p.mu.Unlock()

		found, err := p.fetch(attempt)

		p.mu.Lock()
		p.inflight = false
		if err == nil {
			p.found = found
		}
		p.mu.Unlock()

		if err != nil {
			// Logged here rather than by every caller: one of them is a
			// background goroutine with nowhere to return an error to, and
			// another is a redirect, which cannot carry one.
			p.log.WarnContext(attempt, "the identity provider's SAML metadata could not be read; "+
				"SAML sign-in is unavailable and local sign-in is unaffected",
				slog.String("metadata_url", p.metadataURL),
				slog.String("error", err.Error()))
			return nil, err
		}
		p.log.InfoContext(attempt, "read the identity provider's SAML metadata",
			slog.String("metadata_url", p.metadataURL),
			slog.String("idp_entity_id", found.EntityID))
		return found, nil
	})

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %s did not answer in time: %w",
			ErrUnavailable, p.metadataURL, ctx.Err())
	case answer := <-result:
		if answer.Err != nil {
			return nil, answer.Err
		}
		found, ok := answer.Val.(*crewjam.EntityDescriptor)
		if !ok {
			// Unreachable, and answered rather than asserted: this is inside
			// somebody's sign-in.
			return nil, fmt.Errorf("%w: the metadata fetch returned %T", ErrUnavailable, answer.Val)
		}
		return found, nil
	}
}

// fetch performs one HTTP read of the metadata document.
func (p *Provider) fetch(ctx context.Context) (*crewjam.EntityDescriptor, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not a URL that can be fetched: %w",
			ErrUnavailable, p.metadataURL, err)
	}

	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUnavailable, p.metadataURL, err)
	}
	defer response.Body.Close() //nolint:errcheck // a read-only body

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s answered %s", ErrUnavailable, p.metadataURL, response.Status)
	}

	// LimitReader with one byte of headroom, so that a document at exactly the
	// limit is accepted and one over it is reported rather than silently
	// truncated into XML that does not parse.
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrUnavailable, p.metadataURL, err)
	}
	if len(body) > maxMetadataBytes {
		return nil, fmt.Errorf("%w: %s served more than %d bytes of metadata",
			ErrUnavailable, p.metadataURL, maxMetadataBytes)
	}
	return parseMetadata(body)
}

// parseMetadata turns a metadata document into the descriptor a sign-in is
// validated against, and refuses one that could not be used to validate
// anything.
func parseMetadata(raw []byte) (*crewjam.EntityDescriptor, error) {
	// Before anything reads it: xml-roundtrip-validator refuses a document that
	// Go's encoding/xml would parse differently from the way an XML canonicalizer
	// does. That difference is the whole family of XML signature wrapping
	// attacks — a document where the signature covers one element and the parser
	// hands you another — and github.com/crewjam/saml runs the same check on
	// every assertion. It runs here too because this document decides *which key
	// signs*, which is the one thing worth more than any single assertion.
	if err := xrv.Validate(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("%w: the document is not XML this parser will read the same way "+
			"twice: %w", ErrUnavailable, err)
	}

	descriptor, err := unmarshalDescriptor(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: the document is not SAML metadata: %w", ErrUnavailable, err)
	}
	if descriptor.EntityID == "" {
		return nil, fmt.Errorf("%w: the metadata names no entity ID, so no assertion's issuer could "+
			"ever be checked against it", ErrUnavailable)
	}
	if !hasSigningKey(descriptor) {
		// Refused here rather than at the first assertion. A metadata document
		// with no key is one under which *every* assertion fails to verify, and
		// finding that out at somebody's first login — as "the assertion was
		// rejected" — is finding it out in the worst possible place.
		return nil, fmt.Errorf("%w: %s publishes no signing certificate, so no assertion from it "+
			"could ever be verified", ErrUnavailable, descriptor.EntityID)
	}
	return descriptor, nil
}

// unmarshalDescriptor reads the two shapes an identity provider publishes.
//
// An `<EntityDescriptor>` is one entity and is what most consoles export. An
// `<EntitiesDescriptor>` is a container of them, which is what a federation
// aggregate looks like and what several products export by default — including
// Shibboleth and some Keycloak versions. The one with an IDPSSODescriptor in it
// is the one being configured; an aggregate carrying several is refused rather
// than guessed at, because guessing would pick a signing key by document order.
func unmarshalDescriptor(raw []byte) (*crewjam.EntityDescriptor, error) {
	var entity crewjam.EntityDescriptor
	if err := xml.Unmarshal(raw, &entity); err == nil {
		return &entity, nil
	}

	var entities crewjam.EntitiesDescriptor
	if err := xml.Unmarshal(raw, &entities); err != nil {
		return nil, err
	}

	var providers []*crewjam.EntityDescriptor
	for i := range entities.EntityDescriptors {
		if len(entities.EntityDescriptors[i].IDPSSODescriptors) > 0 {
			providers = append(providers, &entities.EntityDescriptors[i])
		}
	}
	switch len(providers) {
	case 0:
		return nil, errors.New("the EntitiesDescriptor contains no identity provider")
	case 1:
		return providers[0], nil
	default:
		return nil, fmt.Errorf("the EntitiesDescriptor contains %d identity providers; "+
			"extract the one this deployment federates with", len(providers))
	}
}

// hasSigningKey reports whether the metadata carries a certificate an assertion
// could be verified against. A descriptor whose key descriptors have no `use`
// counts: the specification says an unqualified key is good for both, and
// several providers publish exactly that.
func hasSigningKey(descriptor *crewjam.EntityDescriptor) bool {
	for _, idp := range descriptor.IDPSSODescriptors {
		for _, key := range idp.KeyDescriptors {
			if key.Use != "" && key.Use != "signing" {
				continue
			}
			if len(key.KeyInfo.X509Data.X509Certificates) > 0 {
				return true
			}
		}
	}
	return false
}

// readMetadataFile reads a metadata document from disk and checks it parses, so
// that a file which is not metadata is a startup error naming the variable
// rather than a login failure months later.
func readMetadataFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // an operator-supplied configuration path
	if err != nil {
		return nil, fmt.Errorf("saml: read the identity provider metadata at %q: %w", path, err)
	}
	if len(raw) > maxMetadataBytes {
		return nil, fmt.Errorf("saml: the identity provider metadata at %q is %d bytes, and the "+
			"limit is %d", path, len(raw), maxMetadataBytes)
	}
	if _, err := parseMetadata(raw); err != nil {
		return nil, fmt.Errorf("saml: the identity provider metadata at %q: %w", path, err)
	}
	return raw, nil
}

// loadCertificate reads this service provider's certificate.
//
// The first CERTIFICATE block wins, and anything after it is ignored: a PEM file
// from a certificate authority is frequently a chain, and the leaf is what is
// published and what signs.
func loadCertificate(path string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // an operator-supplied configuration path
	if err != nil {
		return nil, fmt.Errorf("saml: read the service provider certificate at %q: %w", path, err)
	}

	for block, rest := pem.Decode(raw); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("saml: the certificate at %q does not parse: %w", path, err)
		}
		return certificate, nil
	}
	return nil, fmt.Errorf("saml: %q holds no PEM CERTIFICATE block", path)
}

// loadKey reads this service provider's private key, in either of the two
// spellings openssl produces.
//
// Only RSA. The XML signature this key makes is RSA-SHA256, which is what every
// identity provider expects from a service provider and the only thing
// goxmldsig signs with — an ECDSA key here would produce a registration nothing
// on the other side can verify, so it is refused with a message that says so
// rather than at the first sign-in.
func loadKey(path string) (crypto.Signer, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // an operator-supplied configuration path
	if err != nil {
		return nil, fmt.Errorf("saml: read the service provider key at %q: %w", path, err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("saml: %q holds no PEM block", path)
	}

	var parsed any
	switch block.Type {
	case "RSA PRIVATE KEY": // PKCS#1, `openssl genrsa`
		parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY": // PKCS#8, `openssl req -newkey`
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return nil, fmt.Errorf("saml: %q holds an elliptic-curve key; SAML signatures here are "+
			"RSA-SHA256, so an RSA key is needed", path)
	default:
		return nil, fmt.Errorf("saml: %q holds a %q block, want an RSA private key", path, block.Type)
	}
	if err != nil {
		// Deliberately not wrapping the parse error's contents further: this is
		// a private key, and the failure is about its encoding.
		return nil, fmt.Errorf("saml: the private key at %q does not parse: %w", path, err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("saml: the private key at %q is a %T; SAML signatures here are "+
			"RSA-SHA256, so an RSA key is needed", path, parsed)
	}
	return key, nil
}

// trimArtifactBinding removes the artifact assertion consumer service the
// library advertises by default.
//
// This deployment implements the HTTP-POST binding and nothing else — the
// assertion consumer takes a form with a `SAMLResponse` in it, and that is what
// api/openapi.yaml describes. Advertising a binding the endpoint does not accept
// would let an identity provider pick it, and the failure would be a sign-in
// that never reaches any code written here.
func trimArtifactBinding(descriptor *crewjam.EntityDescriptor) {
	for i := range descriptor.SPSSODescriptors {
		kept := descriptor.SPSSODescriptors[i].AssertionConsumerServices[:0]
		for _, endpoint := range descriptor.SPSSODescriptors[i].AssertionConsumerServices {
			if endpoint.Binding == crewjam.HTTPPostBinding {
				kept = append(kept, endpoint)
			}
		}
		descriptor.SPSSODescriptors[i].AssertionConsumerServices = kept
	}
}

// marshalMetadata renders the descriptor as the XML document an identity
// provider is given, with a declaration and indentation — it is a document a
// human pastes into a console and reads afterwards to check they pasted the
// right one.
func marshalMetadata(descriptor *crewjam.EntityDescriptor) ([]byte, error) {
	body, err := xml.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
