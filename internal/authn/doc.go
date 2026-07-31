// Package authn establishes who the caller is: local password login, TOTP and
// recovery codes, OIDC, SAML, and scoped service tokens. It produces the
// identity that package authz then makes decisions about.
//
// Implemented by M1-002 through M1-011.
package authn
