package httpapi

import (
	"context"
	"log/slog"

	"github.com/bryanster/purpleops/internal/authn"
	"github.com/bryanster/purpleops/internal/authn/challenge"
	"github.com/bryanster/purpleops/internal/authn/session"
	"github.com/bryanster/purpleops/internal/httpapi/gen"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/version"
)

// handlers implements the generated interface. Its dependencies arrive through
// the struct, so there is no package-level database handle for a later change
// to reach for (PLAN.md §6).
type handlers struct {
	store store.Store

	// auth answers the questions the endpoints in authhandlers.go and
	// mfahandlers.go ask; sessions and challenges are here for the one thing
	// those endpoints do that is transport rather than policy — building the
	// Set-Cookie header.
	auth       *authn.Service
	sessions   *session.Manager
	challenges *challenge.Manager

	log *slog.Logger
}

// The compiler is what keeps this in step with api/openapi.yaml: adding an
// operation to the spec and regenerating breaks this line until it is
// implemented, and removing one breaks the method that is left behind.
var _ gen.StrictServerInterface = (*handlers)(nil)

// GetHealth reports whether this server and its dependencies are answering.
//
// It is public and unauthenticated by design (api/openapi.yaml): a health check
// that needs a session reports "unhealthy" whenever authentication itself
// breaks, which is the one time an orchestrator most needs the truth.
func (h *handlers) GetHealth(ctx context.Context, _ gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	health := gen.Health{
		Status: gen.HealthStateOk,
		Checks: gen.HealthChecks{Db: gen.HealthStateOk},
	}

	if err := h.store.Health(ctx); err != nil {
		// The client is told which check failed and no more — the reason names
		// the database file and the driver's complaint. This is the only record
		// of why, so it is logged at error level even though the response is a
		// documented 503 rather than a fault.
		h.log.ErrorContext(ctx, "health check failed",
			slog.String("check", "db"),
			slog.String("error", err.Error()))
		health.Status = gen.HealthStateError
		health.Checks.Db = gen.HealthStateError
	}

	if health.Status != gen.HealthStateOk {
		// 503 with the same body as the 200: a monitor that only reads the
		// status code gets the right answer, and one that reads the body finds
		// out which dependency is down.
		return gen.GetHealth503JSONResponse(health), nil
	}
	return gen.GetHealth200JSONResponse(health), nil
}

// GetVersion returns the build identity stamped into this binary. Public: the
// SPA shows it on the login page, and a support request is unanswerable without
// it.
func (h *handlers) GetVersion(_ context.Context, _ gen.GetVersionRequestObject) (gen.GetVersionResponseObject, error) {
	info := version.Get()
	return gen.GetVersion200JSONResponse{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildDate: info.BuildDate,
	}, nil
}
