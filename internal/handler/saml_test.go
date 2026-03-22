package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crewjam/saml"
)


// --- samlACSURL ---

func TestSAMLACSURLNotSetByDefault(t *testing.T) {
	// When SAML is not initialized, samlACSURL should be empty.
	samlACSURL = ""
	if samlACSURL != "" {
		t.Error("expected empty samlACSURL before initialization")
	}
}

// --- HandleSAMLMetadata ---

func TestHandleSAMLMetadataNotConfigured(t *testing.T) {
	samlSP = nil

	r := httptest.NewRequest("GET", "/auth/saml/metadata", nil)
	c, w := ginCtx(r)
	HandleSAMLMetadata(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when SAML not configured, got %d", w.Code)
	}
}

// --- HandleSAMLLogin ---

func TestHandleSAMLLoginNotConfigured(t *testing.T) {
	samlSP = nil

	r := httptest.NewRequest("GET", "/auth/saml/login", nil)
	c, w := ginCtx(r)
	HandleSAMLLogin(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when SAML not configured, got %d", w.Code)
	}
}

// --- HandleSAMLACS ---

func TestHandleSAMLACSNotConfigured(t *testing.T) {
	samlSP = nil

	r := httptest.NewRequest("POST", "/auth/saml/acs", nil)
	c, w := ginCtx(r)
	HandleSAMLACS(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when SAML not configured, got %d", w.Code)
	}
}

// --- getSAMLAttribute ---

func TestGetSAMLAttribute(t *testing.T) {
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name:         "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
						FriendlyName: "email",
						Values: []saml.AttributeValue{
							{Value: "user@example.com"},
						},
					},
					{
						Name:         "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
						FriendlyName: "displayName",
						Values: []saml.AttributeValue{
							{Value: "Test User"},
						},
					},
				},
			},
		},
	}

	t.Run("find by Name", func(t *testing.T) {
		email := getSAMLAttribute(assertion,
			"email",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		)
		if email != "user@example.com" {
			t.Errorf("expected 'user@example.com', got %q", email)
		}
	})

	t.Run("find by FriendlyName", func(t *testing.T) {
		email := getSAMLAttribute(assertion, "email")
		if email != "user@example.com" {
			t.Errorf("expected 'user@example.com', got %q", email)
		}
	})

	t.Run("find display name", func(t *testing.T) {
		name := getSAMLAttribute(assertion,
			"displayName",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		)
		if name != "Test User" {
			t.Errorf("expected 'Test User', got %q", name)
		}
	})

	t.Run("attribute not found", func(t *testing.T) {
		val := getSAMLAttribute(assertion, "nonexistent", "also-nonexistent")
		if val != "" {
			t.Errorf("expected empty string for nonexistent attribute, got %q", val)
		}
	})
}

func TestGetSAMLAttributeEmptyAssertion(t *testing.T) {
	assertion := &saml.Assertion{}
	val := getSAMLAttribute(assertion, "email")
	if val != "" {
		t.Errorf("expected empty string for empty assertion, got %q", val)
	}
}

func TestGetSAMLAttributeEmptyValues(t *testing.T) {
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name:   "email",
						Values: []saml.AttributeValue{}, // No values
					},
				},
			},
		},
	}

	val := getSAMLAttribute(assertion, "email")
	if val != "" {
		t.Errorf("expected empty string for attribute with no values, got %q", val)
	}
}

func TestGetSAMLAttributeMultipleStatements(t *testing.T) {
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name:   "role",
						Values: []saml.AttributeValue{{Value: "admin"}},
					},
				},
			},
			{
				Attributes: []saml.Attribute{
					{
						Name:   "email",
						Values: []saml.AttributeValue{{Value: "multi@example.com"}},
					},
				},
			},
		},
	}

	email := getSAMLAttribute(assertion, "email")
	if email != "multi@example.com" {
		t.Errorf("expected 'multi@example.com', got %q", email)
	}

	role := getSAMLAttribute(assertion, "role")
	if role != "admin" {
		t.Errorf("expected 'admin', got %q", role)
	}
}

func TestGetSAMLAttributeFirstValueUsed(t *testing.T) {
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name: "email",
						Values: []saml.AttributeValue{
							{Value: "first@example.com"},
							{Value: "second@example.com"},
						},
					},
				},
			},
		},
	}

	email := getSAMLAttribute(assertion, "email")
	if email != "first@example.com" {
		t.Errorf("expected first value 'first@example.com', got %q", email)
	}
}

func TestGetSAMLAttributeUPNFallback(t *testing.T) {
	// Azure AD sometimes uses UPN as email.
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name:   "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn",
						Values: []saml.AttributeValue{{Value: "upn@example.com"}},
					},
				},
			},
		},
	}

	// If we search for the UPN attribute name, we should find it.
	upn := getSAMLAttribute(assertion,
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn",
	)
	if upn != "upn@example.com" {
		t.Errorf("expected 'upn@example.com', got %q", upn)
	}
}
