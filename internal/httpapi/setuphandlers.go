package httpapi

import (
	"context"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/setup"
)

// The first-run state endpoints. Like every other handler in this package they
// translate and nothing else: what "set up" means, and what finishing it
// writes, live in internal/setup.
//
// Who may call them is not decided here either. api/openapi.yaml maps the two
// operations to `settings.read` and `settings.manage`, and the authorization
// middleware refuses anybody without them before either function below is
// entered (M1-013).

// GetSetupState reports whether this installation has been through the wizard.
func (h *handlers) GetSetupState(ctx context.Context, _ gen.GetSetupStateRequestObject) (gen.GetSetupStateResponseObject, error) {
	state, err := h.setup.State(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetSetupState200JSONResponse(setupState(state)), nil
}

// CompleteSetup records that somebody finished it, and answers with the state
// as stored — including when it was already complete, which is why a retried
// request is safe.
func (h *handlers) CompleteSetup(ctx context.Context, _ gen.CompleteSetupRequestObject) (gen.CompleteSetupResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	state, err := h.setup.Complete(ctx, subject)
	if err != nil {
		return nil, err
	}
	return gen.CompleteSetup200JSONResponse(setupState(state)), nil
}

// setupState renders the state for the wire in one place, so the two responses
// that carry one cannot describe it differently.
func setupState(s setup.State) gen.SetupState {
	out := gen.SetupState{Completed: s.Completed}
	if !s.CompletedAt.IsZero() {
		at := s.CompletedAt
		out.CompletedAt = &at
	}
	if s.CompletedBy != "" {
		out.CompletedBy.Set(s.CompletedBy)
	}
	return out
}
