package saml

import (
	"fmt"
	"strings"

	crewjam "github.com/crewjam/saml"
)

// Reading attributes out of a validated assertion.
//
// It is more forgiving than the rest of this package on purpose, and for the
// reason internal/authn/oidc's claims.go gives: validation is where a provider
// is held to the specification exactly, because that is where being lax is a
// security property being given away. This is where five directories that all
// disagree about how to spell an email address are turned into one struct, and
// being strict here buys nothing but an integration that does not work.
//
// What is *not* forgiving: the `NameID` is required. Everything decided from it
// is decided in internal/authn.

// identityOf turns a validated assertion into the facts this deployment acts on.
func (p *Provider) identityOf(assertion *crewjam.Assertion) (Identity, error) {
	subject := nameID(assertion)
	if subject == "" {
		// Reachable: an assertion may carry attributes and no NameID, and some
		// directories are configured that way by accident. Refused out loud
		// rather than linking an account to the empty string, which would match
		// the empty subject of the next person through the door.
		return Identity{}, fmt.Errorf("%w: the assertion carries no NameID, so there is nothing to "+
			"identify an account by. The identity provider has to be configured to send one",
			ErrRejected)
	}

	attributes := attributesOf(assertion)

	identity := Identity{
		Subject: subject,
		Email:   strings.TrimSpace(first(attributes, p.emailAttributes)),
		Groups:  all(attributes, p.groupAttributes),
	}
	identity.DisplayName = strings.TrimSpace(first(attributes, p.nameAttributes))
	if identity.DisplayName == "" {
		// Something has to be shown next to their comments. The address is what
		// the account is known by anyway, and an empty name would be a blank in
		// every interface that renders one.
		identity.DisplayName = identity.Email
	}
	if identity.Email == "" && isEmail(subject) {
		// A NameID in `emailAddress` format, which is what a directory with no
		// attribute mapping at all produces. Using it is better than refusing a
		// sign-in over a mapping the operator has not made yet, and it is not a
		// weaker claim: it is the same signed document either way.
		identity.Email = subject
	}
	return identity, nil
}

// nameID returns the assertion's subject identifier.
func nameID(assertion *crewjam.Assertion) string {
	if assertion.Subject == nil || assertion.Subject.NameID == nil {
		return ""
	}
	return strings.TrimSpace(assertion.Subject.NameID.Value)
}

// attributesOf flattens every attribute statement into name → values.
//
// Both the `Name` and the `FriendlyName` are keys for the same values, because
// providers disagree about which one carries the meaning: an OID in `Name` with
// `mail` in `FriendlyName` is as common as the reverse. Names are matched
// case-insensitively — `memberOf` and `memberof` are the same attribute in every
// directory that emits either.
//
// Later statements do not overwrite earlier ones, they extend them: an assertion
// that carries `groups` in two statements has all of the groups, not the last
// statement's.
func attributesOf(assertion *crewjam.Assertion) map[string][]string {
	found := map[string][]string{}

	add := func(name string, values []string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || len(values) == 0 {
			return
		}
		found[name] = append(found[name], values...)
	}

	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			values := make([]string, 0, len(attribute.Values))
			for _, value := range attribute.Values {
				if text := strings.TrimSpace(value.Value); text != "" {
					values = append(values, text)
				}
			}
			add(attribute.Name, values)
			if !strings.EqualFold(attribute.FriendlyName, attribute.Name) {
				add(attribute.FriendlyName, values)
			}
		}
	}
	return found
}

// first returns the first value of the first configured name that is present.
// Order is the configuration's, which is what makes the list a preference and
// not a set.
func first(attributes map[string][]string, names []string) string {
	for _, name := range names {
		if values := attributes[strings.ToLower(name)]; len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// all returns every value of the first configured name that is present.
//
// The first name only, not the union of all of them: a directory that sends both
// `groups` and `memberOf` sends the same memberships spelled two ways, and
// concatenating them would map a role from a list with everything in it twice.
// Which of the two is read is the operator's decision, and the order of the list
// is how they make it.
func all(attributes map[string][]string, names []string) []string {
	for _, name := range names {
		if values := attributes[strings.ToLower(name)]; len(values) > 0 {
			return values
		}
	}
	return nil
}

// isEmail is the loosest possible check, and deliberately: it decides whether a
// NameID looks enough like an address to be used as one when the provider sent
// no email attribute. Anything stricter would be this package having an opinion
// about address syntax, which is a fight nobody wins.
func isEmail(value string) bool {
	at := strings.IndexByte(value, '@')
	return at > 0 && at < len(value)-1 && !strings.ContainsAny(value, " \t")
}
