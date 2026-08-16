// The only hand-written file in this package. Everything else is
// client.gen.go, which `make generate` overwrites from api/openapi.yaml.
//
// What is here is what the document cannot say: where `/api/v1` comes from, and
// how a service token is presented. Anything that *can* be expressed in the
// OpenAPI document belongs there instead — a helper here is a second description
// of the API, and the point of this SDK is that there is only one.
package blacklight

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// APIPath is the prefix every operation in this SDK hangs off.
//
// The document declares its one server as the relative URL `/api/v1`, because
// the SPA is served from the same origin as the API and an absolute URL would
// pin every deployment to one host. A generated client cannot send a request to
// a relative URL, so the prefix is applied here instead — see [New].
const APIPath = "/api/v1"

// New returns a client for the Blacklight deployment at baseURL, which is the
// origin an operator would type into a browser — "https://blacklight.example.com"
// — with no API path on it. [APIPath] is appended.
//
// It returns the *WithResponses form deliberately: the plain [Client] hands back
// an *http.Response and leaves the caller to decide what a body means, which is
// the untyped client this repository exists to avoid. Use [NewClient] directly
// if you want the raw one anyway (streaming `text/event-stream` endpoints are
// the honest reason to).
//
// Authentication is not implied. Pass [WithServiceToken] for a service token, or
// a [WithRequestEditorFn] of your own for the session cookie.
func New(baseURL string, opts ...ClientOption) (*ClientWithResponses, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("blacklight: base URL is empty; pass the deployment's origin, such as https://blacklight.example.com")
	}

	// Trailing slashes on both halves would produce `//api/v1`, which some
	// reverse proxies redirect and others 404.
	server := strings.TrimSuffix(baseURL, "/") + APIPath

	return NewClientWithResponses(server, opts...)
}

// WithServiceToken authenticates every request with a service token — the
// `bl_<prefix>_<secret>` string shown once when the token was created.
//
// Service tokens are the credential for scripts and CI. The other one this API
// accepts is the browser session cookie, which this SDK deliberately does not
// help you obtain: a token can be scoped and expired by an administrator, and
// driving the login and MFA endpoints from a program to get a cookie instead is
// working around that.
//
// The token is not validated here. A malformed one fails at the server as a 401
// problem document, which is where a caller can see why.
func WithServiceToken(token string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("blacklight: service token is empty")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}
