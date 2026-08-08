package httpapi

import (
	"context"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// The platform settings endpoints (M1-008). Like every other handler in this
// package they translate and nothing else: what the policy means and what
// changing it does to the sessions already open live in internal/authn.
//
// *Who* may change it is not decided here, or there. api/openapi.yaml maps these
// two operations to `settings.read` and `settings.manage`, and the authorization
// middleware refuses anybody who does not hold them before either function below
// is entered (M1-013). A handler in this package cannot make a role decision:
// TestNoHandlerDecidesForItself fails the build if one of these files so much as
// imports internal/authz.

// GetMfaPolicy reports the platform-wide MFA policy.
func (h *handlers) GetMfaPolicy(ctx context.Context, _ gen.GetMfaPolicyRequestObject) (gen.GetMfaPolicyResponseObject, error) {
	policy, err := h.auth.MFAPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetMfaPolicy200JSONResponse(mfaPolicy(policy)), nil
}

// SetMfaPolicy replaces it, and answers with what is now stored.
//
// The response is the policy rather than a 204 so that an interface which has
// just written one renders from the server's answer instead of from what it
// sent. The two agree today; a future setting whose value is adjusted on the way
// in would make them differ, and a client built against 204 would not notice.
func (h *handlers) SetMfaPolicy(ctx context.Context, request gen.SetMfaPolicyRequestObject) (gen.SetMfaPolicyResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	stored, err := h.auth.SetMFAPolicy(ctx, subject, authn.MFAPolicy{
		RequiredForAll:    request.Body.RequiredForAll,
		RequiredForAdmins: request.Body.RequiredForAdmins,
	})
	if err != nil {
		return nil, err
	}
	return gen.SetMfaPolicy200JSONResponse(mfaPolicy(stored)), nil
}

// mfaPolicy renders a policy for the wire, in one place, so the two responses
// that carry one cannot describe it differently.
func mfaPolicy(policy authn.MFAPolicy) gen.MFAPolicy {
	return gen.MFAPolicy{
		RequiredForAll:    policy.RequiredForAll,
		RequiredForAdmins: policy.RequiredForAdmins,
	}
}
