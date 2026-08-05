package api

// What each operation requires of its caller, declared in openapi.yaml and read
// back out here.
//
// PLAN.md §4: "One function … called from one middleware. No handler makes its
// own role decision." The middleware (M1-013) needs to know, for the operation a
// request resolved to, which action to ask authz.Can about and where in the
// request the thing being acted on is named. That mapping lives in the
// specification, beside the endpoint it protects, rather than in a table beside
// the router — a table beside the router is a second list of the endpoints, and
// a second list is a list with a gap in it. v1's /manage/access was that gap.
//
// So the gap is what this file exists to make impossible. [Requirements] refuses
// a document in which any operation is silent about its caller, internal/httpapi
// calls it while building the server, and api's own tests call it in CI. An
// unmapped endpoint is a build failure and then a server that does not start;
// it is never an endpoint that quietly lets everybody through.

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bryanster/blacklight/internal/authz"
)

// The vendor extensions an operation declares. Exactly one of [extAction],
// [extPublic] and [extSelf] on every operation — see [Requirement].
const (
	// extAction names the authz action, in its wire spelling:
	// `x-authz-action: settings.read`.
	extAction = "x-authz-action"

	// extResource says where the thing being acted on is:
	// `x-authz-resource: {type: engagement, engagement: engagementId}`.
	// Required with, and only with, extAction.
	extResource = "x-authz-resource"

	// extPublic is `x-authz-public: true`: no credential and no permission.
	extPublic = "x-authz-public"

	// extSelf is `x-authz-self: true`: a signed-in caller, acting on their own
	// account, and no permission beyond having signed in.
	extSelf = "x-authz-self"

	// extBecause carries the one-line argument for extPublic or extSelf. This
	// codebase's other two exemption lists — csrfExemptRoutes and
	// enrolmentOnlyRoutes in internal/httpapi — hold every entry to a written
	// reason, for the same reason: an exemption nobody had to justify is one
	// nobody reviewed.
	extBecause = "x-authz-because"
)

// The keys inside [extResource]. Anything else there is a typo, and a typo that
// silently dropped a constraint would be an endpoint protecting less than its
// author believed — so an unknown key is an error rather than an ignored key.
const (
	resourceType       = "type"
	resourceParam      = "param"
	resourceEngagement = "engagement"
)

// Requirement is what one operation requires of its caller.
//
// Exactly one of Public, Self and Action is set on every Requirement
// [Requirements] returns. There is no fourth state, and no zero value reaches
// the map: an operation that declares nothing is an error, never a default.
type Requirement struct {
	// OperationID is the operation this applies to, and the key it is filed
	// under. The middleware resolves a request to its operation through the
	// same kin-openapi router that validates it, and looks the result up here.
	OperationID string

	// Method and Path are where the operation lives in the document, so that a
	// failure names something a reader can go and find.
	Method string
	Path   string

	// Public is an operation that needs no credential and no permission: the
	// health check, the build identity, and the sign-in exchanges that issue
	// the credential everything else needs.
	Public bool

	// Self is an operation that needs a signed-in caller and nothing more,
	// because it acts on that caller's own account and has no way to name
	// anybody else's — TestASelfOperationCannotNameAnotherResource holds it to
	// that by refusing it a path parameter.
	//
	// It is not a hole in "one place decides". The policy question for these is
	// a constant — everyone signed in holds them, over their own account — and
	// a constant is not a decision. What would be a hole is an operation that
	// acquired one by omission, which is why this is a declaration the document
	// has to carry and the server checks at boot.
	Self bool

	// Because is the written argument for Public or Self, and is empty for an
	// operation that names an action.
	Because string

	// Action is the permission [authz.Can] is asked about, and is
	// [authz.ActionUnknown] when Public or Self.
	Action authz.Action

	// Resource locates the thing the action acts on, and is the zero value when
	// Public or Self.
	Resource ResourceRef

	// SessionOnly is true when the operation's security requirement accepts a
	// cookie session and not a bearer service token. The middleware then refuses
	// MethodServiceToken even when the action's rule would allow a token
	// (M2-004 SSE). Distinct from [authz.GuardSessionOnly] on the rule itself:
	// content.sync stays token-reachable for sync start/cancel; only the stream
	// is cookie-bound.
	SessionOnly bool
}

// ResourceRef says where in a request the resource being acted on is named. It
// is the request-shaped half of [authz.Resource]: the middleware turns one into
// the other by reading the path parameters and loading the ownership facts.
type ResourceRef struct {
	// Type is the kind of thing acted on, and must be the type the action's
	// rule in internal/authz declares — [Requirements] checks that, so a
	// mapping that would be denied on every request for naming the wrong type
	// is a startup failure instead.
	Type authz.ResourceType

	// Param names the path parameter carrying the resource's own identifier. It
	// is empty for a resource the operation does not identify — the
	// installation itself, or the shared content library — and, for
	// `type: engagement`, when the identifier is the engagement's own and
	// [ResourceRef.Engagement] already names it.
	Param string

	// Engagement names the path parameter carrying the owning engagement's
	// identifier, and is required for every engagement-scoped type: a resource
	// that cannot say which engagement owns it is one [authz.Can] denies rather
	// than treats as unowned, and "unowned" would mean "no membership needed".
	Engagement string
}

// Requirements returns what every operation in doc requires of its caller,
// keyed by operationId.
//
// It reports every problem it finds rather than the first, for the reason
// config.Load does: somebody fixing a mapping wants the whole list, not one
// round trip per mistake.
func Requirements(doc *openapi3.T) (map[string]Requirement, error) {
	if doc == nil || doc.Paths == nil || doc.Paths.Len() == 0 {
		return nil, errors.New("api: the document describes no operations, so there is nothing to authorize")
	}

	requirements := make(map[string]Requirement, doc.Paths.Len())
	var problems []string

	for _, path := range slices.Sorted(maps.Keys(doc.Paths.Map())) {
		item := doc.Paths.Value(path)
		operations := item.Operations()
		for _, method := range slices.Sorted(maps.Keys(operations)) {
			requirement, err := requirementOf(method, path, item, operations[method])
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			requirements[requirement.OperationID] = requirement
		}
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf(
			"api: %d operation(s) in openapi.yaml do not say what they require of their caller:\n  - %s",
			len(problems), strings.Join(problems, "\n  - "))
	}
	return requirements, nil
}

// requirementOf reads one operation's declaration, or says what is wrong with
// it. Every return path here is a refusal; there is no branch that produces a
// permissive default, because a permissive default is the defect this whole
// mechanism is built against.
func requirementOf(method, path string, item *openapi3.PathItem, op *openapi3.Operation) (Requirement, error) {
	where := method + " " + path
	if op.OperationID == "" {
		return Requirement{}, fmt.Errorf(
			"%s has no operationId, so a request that resolved to it could not be looked up", where)
	}
	where = fmt.Sprintf("%s (%s)", op.OperationID, where)

	if err := onlyKnownExtensions(where, op.Extensions); err != nil {
		return Requirement{}, err
	}

	public, err := flag(where, op.Extensions, extPublic)
	if err != nil {
		return Requirement{}, err
	}
	self, err := flag(where, op.Extensions, extSelf)
	if err != nil {
		return Requirement{}, err
	}
	action, hasAction := op.Extensions[extAction]

	switch {
	case public && self:
		return Requirement{}, fmt.Errorf("%s declares both %s and %s; an operation requires one thing of its caller",
			where, extPublic, extSelf)
	case (public || self) && hasAction:
		return Requirement{}, fmt.Errorf("%s names an action and also claims an exemption; drop one", where)
	case !public && !self && !hasAction:
		return Requirement{}, fmt.Errorf(
			"%s declares no %s, and no %s or %s exemption. Absence is not permission: say which action it needs, "+
				"or say why it needs none", where, extAction, extPublic, extSelf)
	}

	requirement := Requirement{OperationID: op.OperationID, Method: method, Path: path, Public: public, Self: self}

	if public || self {
		if requirement.Because, err = because(where, op.Extensions); err != nil {
			return Requirement{}, err
		}
		if self {
			// A self operation acts on the caller and can act on nothing else,
			// which is only true while it has no way to name anything else. A
			// path parameter is that way.
			if named := pathParameters(item, op); len(named) > 0 {
				return Requirement{}, fmt.Errorf(
					"%s claims %s but takes the path parameter(s) %s; an operation that can name another resource "+
						"needs an action, not an exemption", where, extSelf, strings.Join(named, ", "))
			}
		}
		return requirement, nil
	}

	if err := hasCredential(where, op, requirement); err != nil {
		return Requirement{}, err
	}
	if requirement.Action, err = parseAction(where, action); err != nil {
		return Requirement{}, err
	}
	if requirement.Resource, err = parseResource(where, item, op, requirement.Action); err != nil {
		return Requirement{}, err
	}
	requirement.SessionOnly = sessionOnlySecurity(op)
	return requirement, nil
}

// sessionOnlySecurity reports whether the operation accepts a cookie session
// and nothing else. An explicit operation-level security list that names only
// cookieSession is the signal; inheriting the document default (cookie OR
// bearer) is not.
func sessionOnlySecurity(op *openapi3.Operation) bool {
	if op.Security == nil || len(*op.Security) == 0 {
		return false
	}
	sawCookie := false
	for _, req := range *op.Security {
		// Each requirement is an OR alternative. Any alternative that is not
		// exactly cookieSession means a non-cookie credential is acceptable.
		if len(req) != 1 {
			return false
		}
		if _, ok := req["cookieSession"]; !ok {
			return false
		}
		sawCookie = true
	}
	return sawCookie
}

// hasCredential refuses an operation that requires a permission while declaring
// that it takes no credential. Nobody could ever satisfy it: authorization needs
// a subject, and `security: []` says there will not be one.
//
// It is checked here rather than left to a reviewer because the failure it
// catches looks like a working endpoint — every request answers 401, which reads
// as "you are not signed in" rather than as "this endpoint is impossible".
func hasCredential(where string, op *openapi3.Operation, requirement Requirement) error {
	if op.Security == nil || len(*op.Security) > 0 {
		return nil
	}
	return fmt.Errorf("%s declares `security: []` but requires %s; an operation with no credential has no subject "+
		"to authorize, so every request to it would be refused", where, requirement.Action)
}

// onlyKnownExtensions refuses an x-authz-* key this file does not implement. A
// misspelled key is otherwise a constraint that silently does not apply, and the
// endpoint protects less than its author believed while looking annotated.
func onlyKnownExtensions(where string, extensions map[string]any) error {
	known := []string{extAction, extResource, extPublic, extSelf, extBecause}
	for _, key := range slices.Sorted(maps.Keys(extensions)) {
		if strings.HasPrefix(key, "x-authz") && !slices.Contains(known, key) {
			return fmt.Errorf("%s declares %s, which is not one of %s",
				where, key, strings.Join(known, ", "))
		}
	}
	return nil
}

// flag reads one boolean exemption. `false` is refused rather than read as
// absence: somebody who wrote it meant something, and the two readings — "not
// exempt" and "I turned this off" — should not be the same silent state.
func flag(where string, extensions map[string]any, key string) (bool, error) {
	raw, ok := extensions[key]
	if !ok {
		return false, nil
	}
	value, ok := raw.(bool)
	switch {
	case !ok:
		return false, fmt.Errorf("%s declares %s: %v, want the literal true", where, key, raw)
	case !value:
		return false, fmt.Errorf("%s declares %s: false. Remove the key instead — an exemption that is written down "+
			"and switched off reads as a decision somebody made and then hid", where, key)
	}
	return true, nil
}

// because reads the written argument for an exemption.
func because(where string, extensions map[string]any) (string, error) {
	raw, ok := extensions[extBecause]
	if !ok {
		return "", fmt.Errorf("%s claims an exemption with no %s; say in one line why this operation needs no "+
			"permission, so the next reader can disagree with the argument rather than guess at it", where, extBecause)
	}
	reason, ok := raw.(string)
	switch {
	case !ok:
		return "", fmt.Errorf("%s declares %s: %v, want a one-line string", where, extBecause, raw)
	case strings.TrimSpace(reason) == "":
		return "", fmt.Errorf("%s declares an empty %s", where, extBecause)
	case strings.Contains(reason, "\n"):
		return "", fmt.Errorf("%s declares a multi-line %s; one line, so it renders in a table", where, extBecause)
	}
	return reason, nil
}

// parseAction resolves the wire name to the constant in internal/authz. A name
// that package does not know is refused: an operation mapped to an action that
// does not exist is an unprotected operation, because [authz.Can] would be asked
// about a verb no rule covers.
func parseAction(where string, raw any) (authz.Action, error) {
	name, ok := raw.(string)
	if !ok {
		return authz.ActionUnknown, fmt.Errorf("%s declares %s: %v, want an action name such as settings.read",
			where, extAction, raw)
	}
	action, known := authz.ParseAction(name)
	if !known {
		return authz.ActionUnknown, fmt.Errorf("%s requires the action %q, which internal/authz does not define. "+
			"Add a row to the rule table there, or fix the name", where, name)
	}
	return action, nil
}

// parseResource reads x-authz-resource and checks it against the action's rule
// and against the operation's own path parameters.
//
// Three things have to line up, and each has been a real bug somewhere: the type
// must be the one the action acts on, an engagement-scoped resource must say
// which engagement owns it, and every parameter named here must be one the
// operation actually has. A mapping that fails any of them would be denied on
// every request — quietly, and only once somebody tried the endpoint.
func parseResource(where string, item *openapi3.PathItem, op *openapi3.Operation, action authz.Action) (ResourceRef, error) {
	raw, ok := op.Extensions[extResource]
	if !ok {
		return ResourceRef{}, fmt.Errorf("%s requires %s but declares no %s, so nothing says what it acts on",
			where, action, extResource)
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return ResourceRef{}, fmt.Errorf("%s declares %s: %v, want a mapping with a %s key",
			where, extResource, raw, resourceType)
	}
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if key != resourceType && key != resourceParam && key != resourceEngagement {
			return ResourceRef{}, fmt.Errorf("%s declares %s.%s, which is not one of %s, %s, %s",
				where, extResource, key, resourceType, resourceParam, resourceEngagement)
		}
	}

	ref := ResourceRef{}
	name, err := field(where, fields, resourceType, true)
	if err != nil {
		return ResourceRef{}, err
	}
	ref.Type = authz.ResourceType(name)
	if ref.Param, err = field(where, fields, resourceParam, false); err != nil {
		return ResourceRef{}, err
	}
	if ref.Engagement, err = field(where, fields, resourceEngagement, false); err != nil {
		return ResourceRef{}, err
	}

	// The action already knows what it acts on. Naming it again in the document
	// is for whoever reads the endpoint rather than the rule table — and this is
	// what stops the two readings from disagreeing.
	if want := resourceOf(action); ref.Type != want {
		return ResourceRef{}, fmt.Errorf("%s acts on a %s according to %s, and %s acts on a %s",
			where, ref.Type, extResource, action, want)
	}

	switch scoped := ref.Type.EngagementScoped(); {
	case scoped && ref.Engagement == "":
		return ResourceRef{}, fmt.Errorf("%s acts on a %s, which belongs to an engagement, and names no %s "+
			"path parameter. A resource that cannot say which engagement owns it is one nobody needs a membership for",
			where, ref.Type, resourceEngagement)
	case !scoped && ref.Engagement != "":
		return ResourceRef{}, fmt.Errorf("%s acts on a %s, which belongs to the installation and not to an "+
			"engagement, and names the %s parameter %q", where, ref.Type, resourceEngagement, ref.Engagement)
	}

	declared := pathParameters(item, op)
	for key, named := range map[string]string{resourceParam: ref.Param, resourceEngagement: ref.Engagement} {
		if named != "" && !slices.Contains(declared, named) {
			return ResourceRef{}, fmt.Errorf("%s reads its %s from the path parameter %q, which it does not declare "+
				"(it has %s)", where, key, named, parameterList(declared))
		}
	}
	return ref, nil
}

// field reads one string out of the resource mapping.
func field(where string, fields map[string]any, key string, required bool) (string, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return "", fmt.Errorf("%s declares %s with no %s", where, extResource, key)
		}
		return "", nil
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s declares %s.%s: %v, want a non-empty string", where, extResource, key, raw)
	}
	return value, nil
}

// resourceOf returns the resource type an action acts on, from the one rule that
// defines it. It reads the table rather than keeping a copy, so an action whose
// resource changes cannot leave a stale expectation here.
func resourceOf(action authz.Action) authz.ResourceType {
	for _, rule := range authz.Rules() {
		if rule.Action == action {
			return rule.Resource
		}
	}
	// Unreachable: parseAction resolved this name through the same table.
	return ""
}

// pathParameters returns the names of the path parameters an operation has,
// counting the ones declared on the path item, which apply to every method on
// it. Sorted, so a failure message reads the same way on every run.
func pathParameters(item *openapi3.PathItem, op *openapi3.Operation) []string {
	var names []string
	for _, parameters := range []openapi3.Parameters{item.Parameters, op.Parameters} {
		for _, parameter := range parameters {
			if parameter.Value != nil && parameter.Value.In == openapi3.ParameterInPath {
				names = append(names, parameter.Value.Name)
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// parameterList renders the path parameters an operation has, for a message
// about one it does not.
func parameterList(declared []string) string {
	if len(declared) == 0 {
		return "none"
	}
	return strings.Join(declared, ", ")
}
