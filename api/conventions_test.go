package api

// The API's conventions, as tests. `make lint` runs them, so breaking one fails
// the build naming the rule rather than surfacing in review three days later.
//
// Every rule here exists because breaking it costs something specific on the far
// side of the generator — a renamed Go method, an untyped TypeScript field, an
// error a client cannot switch on, an endpoint that forgot to require a session.
// The failure messages say what that cost is; if a rule ever gets in the way,
// argue with the message, not with the linter.

import (
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bryanster/blacklight/internal/authz"
)

const (
	problemMediaType = "application/problem+json"
	problemSchemaRef = "#/components/schemas/Problem"
	sharedResponses  = "#/components/responses/"
)

// nonProblemErrorResponses lists, by "<operationId> <status>", the responses at
// or above 400 that are deliberately *not* problem documents. Every other one
// must be, because a client that has to parse a second error shape per endpoint
// is the v1 failure this API is designed against.
//
// Adding an entry here is a design decision: say why, in one line.
var nonProblemErrorResponses = map[string]string{
	// A monitor reads `checks` to find out what is unhealthy; a problem document
	// would tell it only that something is. The 503 is the endpoint's answer,
	// not an error.
	"getHealth 503": "the degraded health report is the point of the endpoint",
}

// declaredCodePattern picks the problem code out of a shared response's
// description — "`code` is `not_found`" — which is where a reader of the
// document finds out which code goes with which status.
var declaredCodePattern = regexp.MustCompile("`code` is `([a-z_]+)`")

// operationIDPattern is camelCase, verb-first: getEngagement, listSteps,
// patchExecutionDetection. It becomes a Go method name and a TypeScript hook
// name, so anything else produces an identifier someone has to look up.
var operationIDPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// Mismatched formats — `format: uuid` on an integer, or a format nobody
// defined — are rejected by the loader itself, which runs with format
// validation enabled (see load in spec.go). There is deliberately no rule for
// them here: a rule that can never be the thing that fails is noise.

func TestEveryOperationHasAUniqueOperationID(t *testing.T) {
	byID := map[string]string{}

	for _, o := range operations(t) {
		switch {
		case o.op.OperationID == "":
			t.Errorf("%s has no operationId; the generator names the Go method and the TypeScript hook after it", o)
		case !operationIDPattern.MatchString(o.op.OperationID):
			t.Errorf("%s has operationId %q, want camelCase and verb-first (listEngagements, getEngagement, patchExecutionDetection)", o, o.op.OperationID)
		}
		if first, ok := byID[o.op.OperationID]; ok {
			t.Errorf("%s and %s share the operationId %q; the generated code cannot declare the same method twice", first, o, o.op.OperationID)
		}
		byID[o.op.OperationID] = o.String()
	}
}

func TestEveryOperationHasAOneLineSummary(t *testing.T) {
	for _, o := range operations(t) {
		switch {
		case strings.TrimSpace(o.op.Summary) == "":
			t.Errorf("%s has no summary; it is the only description of the endpoint a reader of the generated client ever sees", o)
		case strings.Contains(o.op.Summary, "\n"):
			t.Errorf("%s has a multi-line summary; summaries are rendered on one line — put the detail in `description`", o)
		}
	}
}

func TestEveryOperationIsTaggedWithADeclaredTag(t *testing.T) {
	doc := mustLoad(t)

	declared := map[string]bool{}
	for _, tag := range doc.Tags {
		if declared[tag.Name] {
			t.Errorf("tag %q is declared twice", tag.Name)
		}
		if strings.TrimSpace(tag.Description) == "" {
			t.Errorf("tag %q has no description; it groups endpoints in the docs and namespaces them in the client", tag.Name)
		}
		declared[tag.Name] = true
	}

	for _, o := range operations(t) {
		if len(o.op.Tags) == 0 {
			t.Errorf("%s has no tag; it will land in an unnamed group in the docs and the generated client", o)
		}
		for _, tag := range o.op.Tags {
			if !declared[tag] {
				t.Errorf("%s is tagged %q, which is not in the document's `tags` list; a typo here silently invents a new group", o, tag)
			}
		}
	}
}

// TestEveryOperationDocumentsItsErrors is the one that keeps the error model
// singular. Without it, the first endpoint under deadline pressure invents its
// own error body and every client grows a special case.
func TestEveryOperationDocumentsItsErrors(t *testing.T) {
	for _, o := range operations(t) {
		responses := o.op.Responses.Map()

		if _, ok := responses["500"]; !ok {
			t.Errorf("%s does not document a 500; every operation can fail that way, and a client that has no type for it cannot handle it", o)
		}

		problems := 0
		for _, status := range slices.Sorted(maps.Keys(responses)) {
			response := responses[status]

			if status == "default" {
				t.Errorf("%s uses a `default` response; the convention is explicit status codes, so the generated client has a type per outcome", o)
				continue
			}
			code, err := strconv.Atoi(status)
			if err != nil {
				t.Errorf("%s has the response key %q, which is not a status code", o, status)
				continue
			}

			isProblem := response.Value != nil && response.Value.Content[problemMediaType] != nil
			if isProblem {
				problems++
				if ref := response.Value.Content[problemMediaType].Schema.Ref; ref != problemSchemaRef {
					t.Errorf("%s %d serves %s with schema %q, want %q — there is one error shape", o, code, problemMediaType, ref, problemSchemaRef)
				}
				if !strings.HasPrefix(response.Ref, sharedResponses) {
					t.Errorf("%s %d declares its own problem response inline; reference one of %s* so the code/status pairing stays 1:1 with internal/httpapi/apierr", o, code, sharedResponses)
				}
			}

			if code >= 400 && !isProblem {
				if why, exempt := nonProblemErrorResponses[o.op.OperationID+" "+status]; !exempt {
					t.Errorf("%s %d is an error that is not a problem document; make it one, or add it to nonProblemErrorResponses with the reason", o, code)
				} else if why == "" {
					t.Errorf("%s %d is exempt from the problem shape with no reason given", o, code)
				}
			}
		}

		if problems == 0 {
			t.Errorf("%s documents no problem response at all; a caller has nothing to handle when it fails", o)
		}
	}
}

// TestEveryProblemCodeHasASharedResponse is the document's half of the pairing
// in internal/httpapi/apierr: a code with no response describing it is a status
// no operation can document, and a response naming a code the enum does not
// declare is a status a generated client has no type for.
//
// The tests in that package cover the other half — that every code in the enum
// has a status, and that no two share one.
func TestEveryProblemCodeHasASharedResponse(t *testing.T) {
	doc := mustLoad(t)

	schema := doc.Components.Schemas["ProblemCode"]
	if schema == nil || schema.Value == nil || len(schema.Value.Enum) == 0 {
		t.Fatal("components.schemas.ProblemCode declares no enum; it is the set of failures a client has to handle")
	}

	described := map[string]string{}
	for _, name := range sortedKeys(doc.Components.Responses) {
		response := doc.Components.Responses[name]
		if response.Value == nil || response.Value.Content[problemMediaType] == nil {
			continue
		}
		description := ""
		if response.Value.Description != nil {
			description = *response.Value.Description
		}
		match := declaredCodePattern.FindStringSubmatch(description)
		if match == nil {
			t.Errorf("components.responses.%s is a problem response whose description does not say which code it carries; write \"`code` is `some_code`\"", name)
			continue
		}
		if first, taken := described[match[1]]; taken {
			t.Errorf("components.responses.%s and .%s both claim the code %q; the code/status pairing is 1:1", first, name, match[1])
			continue
		}
		described[match[1]] = name
	}

	declared := map[string]bool{}
	for _, value := range schema.Value.Enum {
		code, ok := value.(string)
		if !ok {
			t.Errorf("the ProblemCode enum contains %v (%T), want a string", value, value)
			continue
		}
		declared[code] = true
		if _, ok := described[code]; !ok {
			t.Errorf("the code %q has no response in components.responses describing it; no operation can document the status it is reported with", code)
		}
	}
	for code, name := range described {
		if !declared[code] {
			t.Errorf("components.responses.%s carries the code %q, which the ProblemCode enum does not declare; a generated client has no type for it", name, code)
		}
	}
}

// TestEverySchemaDeclaresItsType keeps the generators honest: an untyped schema
// becomes `interface{}` in Go and `unknown` in TypeScript, which is exactly the
// hole this API is supposed to close.
func TestEverySchemaDeclaresItsType(t *testing.T) {
	walkSchemas(t, func(where string, s *openapi3.Schema) {
		if !s.Type.IsEmpty() {
			return
		}
		// A composition describes its type through its members.
		if len(s.OneOf)+len(s.AnyOf)+len(s.AllOf) > 0 || s.Not != nil {
			return
		}
		t.Errorf("%s declares no type; it generates as `interface{}` in Go and `unknown` in TypeScript", where)
	})
}

func TestNoSchemaUsesTheOpenAPI30NullableFlag(t *testing.T) {
	walkSchemas(t, func(where string, s *openapi3.Schema) {
		if s.Nullable {
			t.Errorf("%s uses `nullable: true`, which is OpenAPI 3.0 and is ignored by a 3.1 reader; write `type: [%s, \"null\"]` instead",
				where, strings.Join(s.Type.Slice(), ", "))
		}
	})
}

// TestNoRequestBodyAcceptsUnknownFields is a structural version of PLAN.md §4:
// a blue user cannot submit a red field if no such field exists in their request
// type. `additionalProperties: true` puts the field back.
func TestNoRequestBodyAcceptsUnknownFields(t *testing.T) {
	doc := mustLoad(t)

	check := func(where string, ref *openapi3.SchemaRef) {
		walkFrom(ref, where, map[*openapi3.Schema]bool{}, func(at string, s *openapi3.Schema) {
			if s.AdditionalProperties.Has != nil && *s.AdditionalProperties.Has {
				t.Errorf("the request body %s allows additional properties; a request type that accepts unknown fields is how a caller writes a field it has no right to", at)
			}
		})
	}

	for _, name := range sortedKeys(doc.Components.RequestBodies) {
		body := doc.Components.RequestBodies[name]
		for _, mediaType := range sortedKeys(body.Value.Content) {
			check("components.requestBodies."+name+" ("+mediaType+")", body.Value.Content[mediaType].Schema)
		}
	}
	for _, o := range operations(t) {
		if o.op.RequestBody == nil || o.op.RequestBody.Value == nil {
			continue
		}
		for _, mediaType := range sortedKeys(o.op.RequestBody.Value.Content) {
			check(o.String()+" ("+mediaType+")", o.op.RequestBody.Value.Content[mediaType].Schema)
		}
	}
}

// TestTheAPIIsAuthenticatedByDefault protects the document-level `security`
// requirement. With it, an endpoint added without a thought about auth inherits
// "needs a session"; without it, that endpoint is public — which is precisely
// how v1's /manage/access ended up ungated.
func TestTheAPIIsAuthenticatedByDefault(t *testing.T) {
	doc := mustLoad(t)

	if len(doc.Security) == 0 {
		t.Fatal("the document declares no default `security`; every operation added from here on would be public unless someone remembers otherwise")
	}

	declared := doc.Components.SecuritySchemes
	requirements := map[string]openapi3.SecurityRequirements{"the document default": doc.Security}
	for _, o := range operations(t) {
		if o.op.Security != nil {
			requirements[o.String()] = *o.op.Security
		}
	}
	for _, where := range sortedKeys(requirements) {
		for _, requirement := range requirements[where] {
			for _, scheme := range sortedKeys(requirement) {
				if declared[scheme] == nil {
					t.Errorf("%s requires the security scheme %q, which is not declared in components.securitySchemes", where, scheme)
				}
			}
		}
	}
}

// TestEveryPublicOperationSaysWhyItIsPublic makes "no authentication" a decision
// with a written justification rather than an empty list nobody notices in a
// diff.
func TestEveryPublicOperationSaysWhyItIsPublic(t *testing.T) {
	for _, o := range operations(t) {
		if o.op.Security == nil || len(*o.op.Security) > 0 {
			continue
		}
		if strings.TrimSpace(o.op.Description) == "" {
			t.Errorf("%s overrides the default with `security: []` and gives no description; an unauthenticated endpoint has to explain itself", o)
		}
	}
}

// TestPaginationIsDeclaredOnce keeps every collection endpoint on the same page
// contract: `limit` and `cursor` in, `{items, nextCursor}` out. A second
// definition of `limit` with a different maximum is a client that pages
// correctly against one endpoint and not another.
func TestPaginationIsDeclaredOnce(t *testing.T) {
	doc := mustLoad(t)

	limit := doc.Components.Parameters["Limit"]
	if limit == nil || limit.Value == nil {
		t.Fatal("components.parameters.Limit is missing; it is the shared half of the pagination convention")
	}
	if got := limit.Value.Name; got != "limit" {
		t.Errorf("components.parameters.Limit is named %q, want %q", got, "limit")
	}
	if got, want := limit.Value.Schema.Value.Default, 50.0; got != any(want) {
		t.Errorf("the default limit is %v, want %v", got, want)
	}
	switch max := limit.Value.Schema.Value.Max; {
	case max == nil:
		t.Error("the limit has no maximum; an unbounded page size is a denial of service with extra steps")
	case *max != 200:
		t.Errorf("the maximum limit is %v, want 200", *max)
	}

	cursor := doc.Components.Parameters["Cursor"]
	if cursor == nil || cursor.Value == nil {
		t.Fatal("components.parameters.Cursor is missing; it is the other half of the pagination convention")
	}
	if got := cursor.Value.Name; got != "cursor" {
		t.Errorf("components.parameters.Cursor is named %q, want %q", got, "cursor")
	}

	for _, o := range operations(t) {
		for _, parameter := range o.op.Parameters {
			if parameter.Ref != "" || parameter.Value == nil {
				continue
			}
			if name := parameter.Value.Name; name == "limit" || name == "cursor" {
				t.Errorf("%s declares its own %q parameter; reference #/components/parameters/%s instead", o, name, strings.ToUpper(name[:1])+name[1:])
			}
		}
	}
}

// TestTheOnlyServerIsTheVersionedRelativePath pins the base path the generated
// TypeScript client uses. An absolute URL here would send the SPA of every
// deployment to whichever host was in the spec.
func TestTheOnlyServerIsTheVersionedRelativePath(t *testing.T) {
	doc := mustLoad(t)

	if len(doc.Servers) != 1 {
		t.Fatalf("the document declares %d servers, want exactly 1: the SPA is served from the same origin as the API", len(doc.Servers))
	}
	if got, want := doc.Servers[0].URL, "/api/v1"; got != want {
		t.Errorf("server URL = %q, want %q", got, want)
	}
}

// operation is one method of one path, with enough identity to name itself in a
// failure message.
type operation struct {
	method string
	path   string
	op     *openapi3.Operation
}

func (o operation) String() string { return o.method + " " + o.path }

// operations returns every operation in the document, in a stable order so that
// a run's failures always read the same way.
func operations(t *testing.T) []operation {
	t.Helper()

	doc := mustLoad(t)

	var ops []operation
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			ops = append(ops, operation{method: method, path: path, op: op})
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].path != ops[j].path {
			return ops[i].path < ops[j].path
		}
		return ops[i].method < ops[j].method
	})
	return ops
}

func mustLoad(t *testing.T) *openapi3.T {
	t.Helper()

	doc, err := Load()
	if err != nil {
		t.Fatalf("load the embedded spec: %v", err)
	}
	return doc
}

// walkSchemas visits every schema reachable from the document — component
// schemas, parameters, request bodies and responses, inline or referenced —
// exactly once, so a rule written here applies to the whole API rather than to
// the part someone remembered.
func walkSchemas(t *testing.T, visit func(where string, s *openapi3.Schema)) {
	t.Helper()

	doc := mustLoad(t)
	seen := map[*openapi3.Schema]bool{}

	for _, name := range sortedKeys(doc.Components.Schemas) {
		walkFrom(doc.Components.Schemas[name], "components.schemas."+name, seen, visit)
	}
	for _, name := range sortedKeys(doc.Components.Parameters) {
		walkFrom(doc.Components.Parameters[name].Value.Schema, "components.parameters."+name, seen, visit)
	}
	for _, name := range sortedKeys(doc.Components.RequestBodies) {
		walkContent(doc.Components.RequestBodies[name].Value.Content, "components.requestBodies."+name, seen, visit)
	}
	for _, name := range sortedKeys(doc.Components.Responses) {
		walkContent(doc.Components.Responses[name].Value.Content, "components.responses."+name, seen, visit)
	}
	for _, o := range operations(t) {
		for _, parameter := range o.op.Parameters {
			if parameter.Value != nil {
				walkFrom(parameter.Value.Schema, o.String()+" parameter "+parameter.Value.Name, seen, visit)
			}
		}
		if o.op.RequestBody != nil && o.op.RequestBody.Value != nil {
			walkContent(o.op.RequestBody.Value.Content, o.String()+" request body", seen, visit)
		}
		responses := o.op.Responses.Map()
		for _, status := range slices.Sorted(maps.Keys(responses)) {
			if responses[status].Value != nil {
				walkContent(responses[status].Value.Content, o.String()+" "+status, seen, visit)
			}
		}
	}
}

func walkContent(content openapi3.Content, where string, seen map[*openapi3.Schema]bool, visit func(string, *openapi3.Schema)) {
	for _, mediaType := range sortedKeys(content) {
		walkFrom(content[mediaType].Schema, where+" ("+mediaType+")", seen, visit)
	}
}

func walkFrom(ref *openapi3.SchemaRef, where string, seen map[*openapi3.Schema]bool, visit func(string, *openapi3.Schema)) {
	if ref == nil || ref.Value == nil {
		return
	}
	if ref.Ref != "" {
		// Report a referenced schema under the name it is defined by, not under
		// every property that points at it.
		where = ref.Ref
	}
	if seen[ref.Value] {
		return
	}
	seen[ref.Value] = true

	s := ref.Value
	visit(where, s)

	for _, name := range sortedKeys(s.Properties) {
		walkFrom(s.Properties[name], where+"."+name, seen, visit)
	}
	walkFrom(s.Items, where+"[]", seen, visit)
	walkFrom(s.AdditionalProperties.Schema, where+".additionalProperties", seen, visit)
	walkFrom(s.Not, where+".not", seen, visit)
	for i, sub := range s.AllOf {
		walkFrom(sub, where+".allOf["+strconv.Itoa(i)+"]", seen, visit)
	}
	for i, sub := range s.OneOf {
		walkFrom(sub, where+".oneOf["+strconv.Itoa(i)+"]", seen, visit)
	}
	for i, sub := range s.AnyOf {
		walkFrom(sub, where+".anyOf["+strconv.Itoa(i)+"]", seen, visit)
	}
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	return slices.Sorted(maps.Keys(m))
}

// TestTheRoleEnumsMatchTheOnePlaceRolesAreDefined ties the wire vocabulary to
// internal/authz.
//
// The spec owns what a client sees and authz owns what the server decides, and
// they must be the same words. A role added to one and not the other is a
// membership the API can express and the policy cannot read — or, in the
// direction v1 actually failed, a second spelling of "blue".
func TestTheRoleEnumsMatchTheOnePlaceRolesAreDefined(t *testing.T) {
	doc := mustLoad(t)

	platform := make([]string, 0, 2)
	for _, role := range authz.PlatformRoles() {
		platform = append(platform, string(role))
	}
	engagement := make([]string, 0, 4)
	for _, role := range authz.EngagementRoles() {
		engagement = append(engagement, string(role))
	}

	for schema, want := range map[string][]string{
		"PlatformRole":   platform,
		"EngagementRole": engagement,
	} {
		ref, ok := doc.Components.Schemas[schema]
		if !ok || ref.Value == nil {
			t.Errorf("the document has no %s schema, but internal/authz defines the role", schema)
			continue
		}

		got := make([]string, 0, len(ref.Value.Enum))
		for _, value := range ref.Value.Enum {
			role, isString := value.(string)
			if !isString {
				t.Errorf("%s's enum contains %v, which is not a string", schema, value)
				continue
			}
			got = append(got, role)
		}

		if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("%s enumerates %v; internal/authz defines %v. The two must be the same words",
				schema, got, want)
		}
	}
}
