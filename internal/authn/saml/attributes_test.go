package saml

import (
	"slices"
	"testing"

	crewjam "github.com/crewjam/saml"
)

// Reading attributes, which is where five directories that disagree about how to
// spell an email address are turned into one struct. Every case here is a real
// provider's spelling rather than an invented one.

// attributed builds an assertion carrying these attributes and nothing else.
func attributed(nameID string, attributes ...crewjam.Attribute) *crewjam.Assertion {
	return &crewjam.Assertion{
		Subject: &crewjam.Subject{NameID: &crewjam.NameID{Value: nameID}},
		AttributeStatements: []crewjam.AttributeStatement{
			{Attributes: attributes},
		},
	}
}

func attribute(name, friendly string, values ...string) crewjam.Attribute {
	a := crewjam.Attribute{Name: name, FriendlyName: friendly}
	for _, value := range values {
		a.Values = append(a.Values, crewjam.AttributeValue{Value: value})
	}
	return a
}

// reader is a provider configured only enough to read attributes.
func reader() *Provider {
	return &Provider{
		emailAttributes: []string{"email", "mail", "urn:oid:0.9.2342.19200300.100.1.3"},
		nameAttributes:  []string{"displayName", "name", "cn"},
		groupAttributes: []string{"groups", "memberOf"},
	}
}

func TestAnAttributeIsFoundByItsNameOrItsFriendlyName(t *testing.T) {
	t.Parallel()

	tests := map[string]crewjam.Attribute{
		// Keycloak, and anything else that uses the plain name.
		"by name": attribute("email", "", "rowan@example.com"),
		// ADFS and Entra ID, which put an OID or a URL in Name and the readable
		// spelling in FriendlyName.
		"by friendly name": attribute("urn:oid:1.2.3.4.5", "email", "rowan@example.com"),
		// The OID form, which is in the configured list itself.
		"by the OID in the list": attribute(
			"urn:oid:0.9.2342.19200300.100.1.3", "", "rowan@example.com"),
	}
	for name, a := range tests {
		t.Run(name, func(t *testing.T) {
			identity, err := reader().identityOf(attributed("subject", a))
			if err != nil {
				t.Fatalf("identityOf: %v", err)
			}
			if identity.Email != "rowan@example.com" {
				t.Errorf("email = %q, want %q", identity.Email, "rowan@example.com")
			}
		})
	}
}

func TestTheFirstConfiguredNameThatIsPresentWins(t *testing.T) {
	t.Parallel()

	// A directory sending both spellings. The list is a preference, and "email"
	// is ahead of "mail" in it — so reordering the configuration is how an
	// operator changes which one is read.
	identity, err := reader().identityOf(attributed("subject",
		attribute("mail", "", "second@example.com"),
		attribute("email", "", "first@example.com"),
	))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if identity.Email != "first@example.com" {
		t.Errorf("email = %q, want the first configured name to win", identity.Email)
	}
}

func TestGroupsAreReadFromOneAttributeRatherThanUnioned(t *testing.T) {
	t.Parallel()

	// `groups` and `memberOf` carrying the same memberships spelled two ways is
	// the common case, not a rare one. Concatenating them would hand the role
	// mapping a list with everything in it twice.
	identity, err := reader().identityOf(attributed("subject",
		attribute("groups", "", "admins", "everyone"),
		attribute("memberOf", "", "CN=admins,OU=Groups", "CN=everyone,OU=Groups"),
	))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if want := []string{"admins", "everyone"}; !slices.Equal(identity.Groups, want) {
		t.Errorf("groups = %v, want %v", identity.Groups, want)
	}
}

func TestAMultiValuedAttributeKeepsEveryValue(t *testing.T) {
	t.Parallel()

	identity, err := reader().identityOf(attributed("subject",
		attribute("groups", "", "one", "two", "three"),
	))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if want := []string{"one", "two", "three"}; !slices.Equal(identity.Groups, want) {
		t.Errorf("groups = %v, want %v", identity.Groups, want)
	}
}

func TestADisplayNameFallsBackToTheAddress(t *testing.T) {
	t.Parallel()

	// Something has to be shown next to their comments, and a blank is worse
	// than the address they are known by anyway.
	identity, err := reader().identityOf(attributed("subject",
		attribute("email", "", "rowan@example.com"),
	))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if identity.DisplayName != "rowan@example.com" {
		t.Errorf("displayName = %q, want the address", identity.DisplayName)
	}
}

func TestAnEmailAddressNameIDIsUsedWhenNoAttributeCarriesOne(t *testing.T) {
	t.Parallel()

	// A directory with no attribute mapping configured at all, which is what
	// somebody's first attempt looks like. The NameID is the same signed value
	// either way, so using it is not a weaker claim.
	identity, err := reader().identityOf(attributed("rowan@example.com"))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if identity.Email != "rowan@example.com" {
		t.Errorf("email = %q, want the NameID", identity.Email)
	}
}

func TestAnOpaqueNameIDIsNotMistakenForAnAddress(t *testing.T) {
	t.Parallel()

	identity, err := reader().identityOf(attributed("S-1-5-21-1004336348-1177238915"))
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if identity.Email != "" {
		t.Errorf("email = %q, want empty: that NameID is not an address", identity.Email)
	}
}

func TestAnAssertionWithNoNameIDIsRefused(t *testing.T) {
	t.Parallel()

	// Reachable: a directory configured to send attributes and no NameID. An
	// account linked to the empty string would match the empty subject of the
	// next person through the door.
	assertion := &crewjam.Assertion{
		AttributeStatements: []crewjam.AttributeStatement{
			{Attributes: []crewjam.Attribute{attribute("email", "", "rowan@example.com")}},
		},
	}
	if _, err := reader().identityOf(assertion); err == nil {
		t.Fatal("an assertion with no NameID was accepted")
	}
}

func TestAttributesInTwoStatementsAreBothRead(t *testing.T) {
	t.Parallel()

	assertion := &crewjam.Assertion{
		Subject: &crewjam.Subject{NameID: &crewjam.NameID{Value: "subject"}},
		AttributeStatements: []crewjam.AttributeStatement{
			{Attributes: []crewjam.Attribute{attribute("groups", "", "one")}},
			{Attributes: []crewjam.Attribute{attribute("groups", "", "two")}},
		},
	}
	identity, err := reader().identityOf(assertion)
	if err != nil {
		t.Fatalf("identityOf: %v", err)
	}
	if want := []string{"one", "two"}; !slices.Equal(identity.Groups, want) {
		t.Errorf("groups = %v, want %v — a second statement extends the first rather than "+
			"replacing it", identity.Groups, want)
	}
}
