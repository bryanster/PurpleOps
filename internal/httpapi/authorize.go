package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// The one authorization middleware (M1-013). PLAN.md §4: "One function … called
// from one middleware. **No handler makes its own role decision.**"
//
// M1-012 built the function. This is the middleware, and the reason it is a
// middleware is structural rather than tidy: a per-handler check protects the
// handlers somebody remembered, and v1's /manage/access is what the ones nobody
// remembered look like. Here there is one chain, every API route is on it, and
// the mapping it consults is built from api/openapi.yaml at startup — so an
// endpoint with no mapping is a server that does not start rather than an
// endpoint anybody can reach.
//
// What it does not do is decide anything. Every permission question goes to
// [authz.Can]; what this file owns is getting the question right: which
// operation the request is, who is asking, what is being acted on, and what the
// answer means on the wire.
//
// # 403 or 404
//
// A denial that would confirm something the caller may not know is a 404. That
// is not inferred here — the policy sets [authz.Decision.Conceal], because the
// policy is what knows *why* it refused, and a middleware working it out from
// the shape of the subject would be a second place making a permission
// decision. The two cases today, both from PLAN.md §4:
//
//   - A non-member on anything belonging to an engagement. "Non-members get
//     nothing on an engagement, including its existence" — a 403 would confirm
//     the engagement is real, and an identifier that answers 403 while its
//     neighbours answer 404 is an engagement someone has enumerated.
//   - The blue side of a blind engagement on a step that has not been revealed.
//     Learning that a step exists is most of what blind mode withholds.
//
// Everything else is a 403 with `code: "forbidden"` and a fixed detail:
// apierr.Forbidden gives the caller no way to make the response vary, and the
// reason — which names roles and identifiers they may not be entitled to — goes
// to the log and to M1-015's activity log.

// Ownership is what the authorization middleware has to look up before it can
// ask a question about an engagement-scoped resource: which engagement owns the
// thing, whether that engagement runs blind, whether this particular thing has
// been revealed, and what seat the caller holds there.
//
// It is an interface here, and small, for the reason [authz.Can] takes facts
// rather than identifiers: the policy does no I/O, so somebody has to do it, and
// that somebody should be replaceable in a test without a database. M3 supplies
// the implementation along with the engagements themselves.
//
// A build with no implementation is not a build that guesses. [newServer]
// refuses to start when the specification maps any operation to an
// engagement-scoped resource and nothing was given here — so the nil case is
// unreachable rather than defaulted.
type Ownership interface {
	// Facts returns the ownership state of one resource, or [apierr.NotFound]
	// when there is no such thing. Not-found and concealed-denial produce the
	// same response, which is the point: a caller cannot tell them apart.
	Facts(ctx context.Context, ref ResourceRef) (ResourceFacts, error)

	// Seat returns the caller's role in one engagement, and false when they are
	// not a member of it.
	//
	// One engagement, not all of them: authn.Subject deliberately carries no
	// memberships, because loading every membership on every request buys a
	// query for the sake of the handful of requests that name an engagement.
	// This is that handful.
	Seat(ctx context.Context, engagementID, userID string) (authz.EngagementRole, bool, error)
}

// ResourceRef is one resource, identified as the request identified it: the
// type the specification declared, and the identifiers read out of the path.
type ResourceRef struct {
	Type authz.ResourceType

	// ID is the resource's own identifier, and is empty for an operation that
	// names none — one that acts on a collection, or creates something.
	ID string

	// EngagementID is the engagement named in the path. Empty for a
	// platform-scoped resource.
	EngagementID string
}

// ResourceFacts is what a loader found: the ownership state [authz.Can] reads.
// Everything here is a fact about the resource and none of it is about the
// caller, which is what keeps the two loads separate.
type ResourceFacts struct {
	// EngagementID is the engagement that owns the resource. It is loaded
	// rather than taken from the path so that a resource nested under the wrong
	// engagement is denied on the engagement it actually belongs to — the path
	// is the caller's claim, and this is the answer.
	EngagementID string

	// Blind is whether that engagement withholds unrevealed steps from its blue
	// side.
	Blind bool

	// Revealed is whether this particular thing has been shown to the blue
	// side. It is meaningless outside a blind engagement, where everything
	// reads as revealed.
	Revealed bool
}

// Authorization is what the middleware decided about one request. It is put in
// the request context so that the activity log (M1-015) can record who did what,
// to what, and whether they were allowed — from the same values the decision was
// made from, rather than from a handler's reconstruction of them.
type Authorization struct {
	// OperationID is the operation as api/openapi.yaml names it.
	OperationID string

	// Subject is who asked, and is the zero value for a public operation
	// reached anonymously.
	Subject authz.Subject

	// Action is what they asked to do, and is [authz.ActionUnknown] for an
	// operation that needs no permission.
	Action authz.Action

	// Resource is what they asked to do it to, and is the zero value for an
	// operation that acts on no resource.
	Resource authz.Resource

	// Allowed is the outcome, and Reason says why in one line. Reason is for
	// operators: it names roles and identifiers the caller may not be entitled
	// to know about, and nothing writes it into a response.
	Allowed bool
	Reason  string
}

// LogValue renders an authorization for a log line.
func (a Authorization) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("operation", a.OperationID),
		slog.Any("subject", a.Subject),
		slog.String("action", a.Action.String()),
		slog.Any("resource", a.Resource),
		slog.Bool("allowed", a.Allowed),
		slog.String("reason", a.Reason))
}

// authorize refuses every request the policy does not permit, and is the last
// step of the chain described in internal/httpapi/server.go — immediately in
// front of the handlers, and after everything that establishes who the caller
// is.
//
// It is built from doc rather than from a table beside the router: requirements
// come from the specification, which is also what the router that resolves a
// request to its operation is built from. One document, one list of endpoints.
func authorize(doc *openapi3.T, own Ownership, responder *apierr.Responder,
	log *slog.Logger) (func(http.Handler) http.Handler, error) {
	requirements, err := api.Requirements(doc)
	if err != nil {
		return nil, fmt.Errorf("httpapi: %w", err)
	}
	if err := ownershipIsAvailable(requirements, own); err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			requirement, route, err := requirementFor(ctx, requirements)
			if err != nil {
				responder.Write(w, r, err)
				return
			}

			decided, err := decide(ctx, requirement, route, own)
			if err != nil {
				responder.Write(w, r, err)
				return
			}

			log.DebugContext(ctx, "authorization decision", slog.Any("authorization", decided.authorization))
			if !decided.authorization.Allowed {
				// Info rather than warn. A refusal is the system working, and in
				// a browser it is routinely a client asking for something the
				// signed-in user cannot see; a level that cries wolf about that
				// is a level operators turn off. What deserves attention is a
				// pattern of them, which is M1-015's job and not a log level's.
				log.InfoContext(ctx, "refused a request the caller may not make",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("authorization", decided.authorization))
				responder.Write(w, r, decided.refusal)
				return
			}

			next.ServeHTTP(w, r.WithContext(withAuthorization(ctx, decided.authorization)))
		})
	}, nil
}

// ownershipIsAvailable refuses to build a server whose specification maps an
// operation to something an engagement owns while nothing can load the facts
// about it.
//
// This is the same fail-closed discipline as a missing mapping, one level down.
// The alternative — a loader that returns "not implemented" at request time —
// is a 500 in a deployment instead of a failure on the machine of whoever added
// the endpoint, and it makes the "not implemented" branch reachable code that
// has to decide something.
func ownershipIsAvailable(requirements map[string]api.Requirement, own Ownership) error {
	if own != nil {
		return nil
	}
	for _, requirement := range requirements {
		if requirement.Resource.Type.EngagementScoped() {
			return fmt.Errorf(
				"httpapi: %s acts on a %s, which belongs to an engagement, and this server was built with no "+
					"Ownership to load one from (Deps.Ownership)",
				requirement.OperationID, requirement.Resource.Type)
		}
	}
	return nil
}

// requirementFor returns what the operation this request resolved to requires.
//
// Both failures here are faults rather than refusals, and both are unreachable
// on the chain this server builds: the validator runs first and records the
// route, and the mapping is built from the same document the validator's router
// is. They are answered as 500s because a request whose operation cannot be
// identified is one this middleware cannot make a decision about, and the only
// safe thing to do with a decision you cannot make is not to serve it.
func requirementFor(ctx context.Context, requirements map[string]api.Requirement) (api.Requirement, specRoute, error) {
	route, ok := specRouteFrom(ctx)
	if !ok {
		return api.Requirement{}, specRoute{}, apierr.Internal(
			errors.New("the request carries no resolved operation, so the request validator did not run in front of it"))
	}
	requirement, ok := requirements[route.OperationID]
	if !ok {
		return api.Requirement{}, specRoute{}, apierr.Internal(
			fmt.Errorf("no authorization mapping for the operation %q, which the specification router resolved to",
				route.OperationID))
	}
	return requirement, route, nil
}

// decided is one request's outcome: what to record, and — when the answer is no
// — what to send.
type decided struct {
	authorization Authorization
	refusal       error
}

// decide answers one request. The order is the design:
//
//  1. A public operation is allowed before anything is loaded or looked up. It
//     is the only branch that does not require a caller, and the specification
//     is the only thing that can put an operation in it.
//  2. Everything else requires an authenticated caller. A request with none is
//     401 and never 403: "you are not signed in" and "you may not do this" are
//     different instructions to a client, and answering the first with the
//     second is how a session that quietly expired looks like a permissions bug.
//  3. A self operation is allowed there. It acts on the caller's own account and
//     has no way to name anything else — api.Requirements refuses the claim to
//     an operation with a path parameter — so there is no role in the question
//     and nothing for a policy to weigh.
//  4. Everything else loads the facts and asks [authz.Can].
func decide(ctx context.Context, requirement api.Requirement, route specRoute, own Ownership) (decided, error) {
	authorization := Authorization{OperationID: requirement.OperationID}

	if requirement.Public {
		authorization.Allowed = true
		authorization.Reason = "the specification declares this operation public: " + requirement.Because
		return decided{authorization: authorization}, nil
	}

	caller, signedIn := authn.SubjectFrom(ctx)
	if !signedIn {
		return decided{}, apierr.Unauthenticated(
			fmt.Sprintf("%s requires a caller and the request carries no usable session", requirement.OperationID))
	}

	if requirement.Self {
		authorization.Subject = subjectOf(caller, "", "", false)
		authorization.Allowed = true
		authorization.Reason = "the specification declares this operation self-service: " + requirement.Because
		return decided{authorization: authorization}, nil
	}

	resource, subject, err := facts(ctx, requirement, route, caller, own)
	if err != nil {
		return decided{}, err
	}

	// Cookie-only operations (SSE): refuse a service token before asking the
	// policy, so content.sync stays token-reachable for REST while the stream
	// stays session-bound. Distinct from GuardSessionOnly on the rule table.
	if requirement.SessionOnly && subject.Method == authz.MethodServiceToken {
		authorization.Subject = subject
		authorization.Action = requirement.Action
		authorization.Resource = resource
		authorization.Allowed = false
		authorization.Reason = "this operation accepts a session cookie only, not a service token"
		return decided{
			authorization: authorization,
			refusal:       apierr.Forbidden("subscribe to events with a service token"),
		}, nil
	}

	decision := authz.Can(ctx, subject, requirement.Action, resource)

	authorization.Subject = subject
	authorization.Action = requirement.Action
	authorization.Resource = resource
	authorization.Allowed = decision.Allowed
	authorization.Reason = decision.Reason

	if decision.Allowed {
		return decided{authorization: authorization}, nil
	}
	return decided{authorization: authorization, refusal: refuse(requirement, resource, decision)}, nil
}

// refuse turns a denial into the answer the caller gets: 404 where admitting the
// resource exists is itself the leak, and 403 everywhere else. Neither carries
// the reason — see apierr.NotFound and apierr.Forbidden, which put it in the log
// and give the caller a fixed sentence.
func refuse(requirement api.Requirement, resource authz.Resource, decision authz.Decision) error {
	if decision.Conceal {
		return apierr.NotFound(string(resource.Type), concealedID(resource))
	}
	return apierr.Forbidden(fmt.Sprintf("%s on %s %s: %s",
		requirement.Action, resource.Type, concealedID(resource), decision.Reason))
}

// concealedID names the resource for the log line inside a refusal. It is the
// resource's own identifier where the operation named one, and the engagement's
// otherwise — an engagement a non-member asked about is identified by itself.
func concealedID(resource authz.Resource) string {
	if resource.ID != "" {
		return resource.ID
	}
	return resource.EngagementID
}

// facts loads what the decision needs: the resource's ownership state, and the
// caller's seat in the engagement that owns it.
//
// A platform-scoped resource needs neither, and costs no query. That is why the
// settings endpoints — the only mapped ones today — are decided without touching
// the database.
func facts(ctx context.Context, requirement api.Requirement, route specRoute, caller authn.Subject,
	own Ownership) (authz.Resource, authz.Subject, error) {
	// The specification names the path parameters; the router extracted their
	// values. Neither is read anywhere else, so there is no second opinion
	// about which segment of the URL is the engagement.
	ref := ResourceRef{
		Type:         requirement.Resource.Type,
		ID:           route.PathParams[requirement.Resource.Param],
		EngagementID: route.PathParams[requirement.Resource.Engagement],
	}
	// `x-authz-resource: {type: engagement, engagement: engagementId}` names the
	// engagement once, and it is both the resource and its owner.
	if ref.ID == "" && ref.Type == authz.ResourceEngagement {
		ref.ID = ref.EngagementID
	}

	if !ref.Type.EngagementScoped() {
		return authz.Resource{Type: ref.Type, ID: ref.ID}, subjectOf(caller, "", "", false), nil
	}

	loaded, err := own.Facts(ctx, ref)
	if err != nil {
		return authz.Resource{}, authz.Subject{}, err
	}
	seat, member, err := own.Seat(ctx, loaded.EngagementID, caller.UserID)
	if err != nil {
		return authz.Resource{}, authz.Subject{}, err
	}

	resource := authz.Resource{
		Type:            ref.Type,
		ID:              ref.ID,
		EngagementID:    loaded.EngagementID,
		EngagementBlind: loaded.Blind,
		Revealed:        loaded.Revealed,
	}
	if !member {
		// Absence from the map is non-membership, which authz.Can conceals.
		// Saying so here rather than putting an empty role in the map matters:
		// an unrecognised role holds nothing, but it is also not the same fact,
		// and the reason on the denial should say "not a member".
		return resource, subjectOf(caller, "", "", false), nil
	}
	return resource, subjectOf(caller, loaded.EngagementID, seat, true), nil
}

// subjectOf translates the authenticated caller into the subject the policy
// reads, with at most one membership: the engagement this request named.
//
// It is the only place the two Subject types meet. authn's carries what
// authentication established; authz's carries what a rule may read, and the gap
// between them is deliberate — memberships are loaded per request for the one
// engagement in question, and MFA state travels so that a decision's audit line
// records how strong the session that got it was.
//
// A service token's scopes and its engagement binding travel too, and they are
// carried rather than derived: they are what M1-011's second and third fences
// are made of, and a middleware that recomputed either from the request would be
// a second answer to a question authentication already answered. Both are empty
// for a session, which is what makes the fences apply to tokens alone.
func subjectOf(caller authn.Subject, engagementID string, seat authz.EngagementRole, member bool) authz.Subject {
	subject := authz.Subject{
		UserID:            caller.UserID,
		PlatformRole:      caller.PlatformRole,
		Method:            caller.Method,
		TokenScopes:       caller.TokenScopes,
		TokenEngagementID: caller.TokenEngagementID,
		MFASatisfied:      caller.MFASatisfied,
	}
	if member {
		subject.Memberships = map[string]authz.EngagementRole{engagementID: seat}
	}
	return subject
}

// authorizationKey is this file's context key, its own type for the reason every
// other one in this package is.
type authorizationKey struct{}

func withAuthorization(ctx context.Context, authorization Authorization) context.Context {
	return context.WithValue(ctx, authorizationKey{}, authorization)
}

// authorizationFrom returns what was decided about this request, and false for a
// request that did not pass through [authorize] — which, on this server, means a
// request that never reached a handler.
//
// M1-015 is the reader: an activity entry says who did what to what, and the
// values it needs are the ones the decision was made from. A handler
// reconstructing them would be a second answer to a question already answered.
func authorizationFrom(ctx context.Context) (Authorization, bool) {
	authorization, ok := ctx.Value(authorizationKey{}).(Authorization)
	return authorization, ok
}
