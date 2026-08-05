package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/version"
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

	// oidc and saml are the configured single sign-on providers, and each is nil
	// on a deployment with none — which is the default for both, and which the
	// endpoints in oidchandlers.go and samlhandlers.go answer 404 for rather
	// than guessing at. A deployment may have either, both, or neither.
	oidc *oidc.Provider
	saml *saml.Provider

	// activity is the append-only log (M1-015). Nil only in tests that never
	// hit the list endpoints.
	activity *events.Log

	// content is the source registry (M2-002).
	content *content.Registry

	// runner is the global content job worker (M2-003). Nil only in tests that
	// never hit sync/job endpoints.
	runner *content.Runner

	// objects is the ATT&CK object library (M2-006).
	objects *storecontent.Objects

	// procedures is the Atomic / custom procedure library (M2-008).
	procedures *storecontent.Procedures

	// detections is the Sigma / custom detection rule library (M2-009).
	detections *storecontent.Detections

	// emulationPlans is the CTID emulation plan catalog (M2-010).
	emulationPlans *storecontent.EmulationPlans

	// custom is the user-authored content surface (M2-011).
	custom *content.Custom

	// attackpin is the ATT&CK version catalog and pin surface (M2-007).
	attackpin *attackpin.Service

	// hub fans ephemeral UI events (M2-004). Nil only in tests that never hit
	// GET /events.
	hub *events.Hub

	// eventsHeartbeat is how often the SSE handler writes a comment frame.
	eventsHeartbeat time.Duration

	// signInURL is where this deployment is signed in to, absolute, for the one
	// response that has to tell somebody who is not looking at the application:
	// the invite link on `POST /users` (M1-016). There is no mail transport, so
	// an administrator passes it on themselves.
	//
	// It is built once from the configured base URL rather than from a request's
	// Host header — a link derived from a header is a link an attacker can
	// choose.
	signInURL string

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
