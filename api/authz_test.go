package api

// The CI half of M1-013's "adding a new endpoint without an action mapping fails
// CI, not production". The other half is internal/httpapi, which calls
// [Requirements] while building the server and refuses to start without it —
// these tests are what make the failure arrive three minutes after the commit
// rather than at the first request in a deployment.

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bryanster/blacklight/internal/authz"
)

// TestEveryOperationSaysWhatItRequiresOfItsCaller is the rule itself: the real
// document, every operation, no gaps.
func TestEveryOperationSaysWhatItRequiresOfItsCaller(t *testing.T) {
	doc := mustLoad(t)

	requirements, err := Requirements(doc)
	if err != nil {
		t.Fatalf("the authorization mapping is not complete:\n%v", err)
	}

	for _, o := range operations(t) {
		requirement, ok := requirements[o.op.OperationID]
		if !ok {
			t.Errorf("%s has no requirement; Requirements returned no error, so this is a bug in Requirements", o)
			continue
		}
		if got, want := requirement.Method+" "+requirement.Path, o.String(); got != want {
			t.Errorf("%s is filed under %q", o, got)
		}
	}
	if got, want := len(requirements), len(operations(t)); got != want {
		t.Errorf("Requirements returned %d entries for %d operations", got, want)
	}
}

// TestTheMappingIsAnExemptionOrAPermissionAndNeverBoth walks what the document
// actually claims, so that a reader of the test — and of the failure — can see
// the exemptions in one list rather than by grepping the YAML.
//
// It is deliberately not a golden file. Adding an endpoint should not fail a
// test; adding an *exempt* endpoint should, and does, because the count below
// names every exemption it knows about.
func TestTheMappingIsAnExemptionOrAPermissionAndNeverBoth(t *testing.T) {
	requirements, err := Requirements(mustLoad(t))
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	// Every operation that needs no permission, with the reason it gives. A new
	// entry here is a new endpoint somebody may reach without one, which is
	// exactly the change worth stopping a reviewer on.
	exempt := map[string]bool{
		"getHealth": true, "getVersion": true, "login": true, "logout": true,
		"verifyTotp": true, "verifyRecoveryCode": true,
		"getCurrentUser": true, "changePassword": true, "enrollTotp": true,
		"confirmTotp": true, "regenerateRecoveryCodes": true, "disableTotp": true,
		// The single sign-on half of the same door (M1-009): the list of
		// buttons the login page draws, and the two halves of an exchange that
		// issues the credential everything else needs.
		"getAuthProviders": true, "startOidcSignIn": true, "completeOidcSignIn": true,
		// The SAML half of it (M1-010): the same two halves of the same
		// exchange, plus the metadata an identity provider administrator has to
		// be able to fetch before anybody here has an account to fetch it with.
		"getSamlMetadata": true, "startSamlSignIn": true, "completeSamlSignIn": true,
	}

	for id, requirement := range requirements {
		switch {
		case requirement.Public || requirement.Self:
			if !exempt[id] {
				t.Errorf("%s requires no permission (%s), and this test did not know about it. If that is right, "+
					"add it to the list above; the point of the list is that somebody reads the reason",
					id, exemptionOf(requirement))
			}
			if requirement.Action != authz.ActionUnknown {
				t.Errorf("%s is exempt and also names the action %s", id, requirement.Action)
			}
			if strings.TrimSpace(requirement.Because) == "" {
				t.Errorf("%s is exempt with no reason; Requirements should have refused it", id)
			}
		default:
			if exempt[id] {
				t.Errorf("%s is listed as needing no permission and requires %s; remove it from the list",
					id, requirement.Action)
			}
			if requirement.Action == authz.ActionUnknown {
				t.Errorf("%s requires no action and claims no exemption; Requirements should have refused it", id)
			}
			if requirement.Because != "" {
				t.Errorf("%s names an action and also carries a reason for an exemption it does not claim", id)
			}
		}
	}
}

// exemptionOf names which exemption a requirement claims, for a failure message.
func exemptionOf(requirement Requirement) string {
	if requirement.Public {
		return extPublic
	}
	return extSelf
}

// TestRequirementsRefusesAGapOrAMistake breaks the *real* document in the ways a
// careless edit plausibly would and asserts that each is refused, naming the
// problem.
//
// A mutation that no longer changes the spec fails the test rather than passing
// quietly — the same guard TestLoadRejectsABrokenSpec uses, and for the same
// reason: a rule that has stopped being exercised is worse than one nobody
// wrote, because the build is still green about it.
func TestRequirementsRefusesAGapOrAMistake(t *testing.T) {
	cases := map[string]struct {
		break_ func(spec string) string
		want   string
	}{
		"an operation with no mapping at all": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.read\n"+
					"      x-authz-resource:\n        type: platform\n", "", 1)
			},
			want: "declares no x-authz-action",
		},
		"an exemption with no argument for it": {
			break_: func(spec string) string {
				return strings.Replace(spec,
					"      x-authz-because: the login page shows it, and it names software whose source is public anyway.\n",
					"", 1)
			},
			want: "claims an exemption with no x-authz-because",
		},
		"an exemption written down and switched off": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-public: true\n      x-authz-because: the login page",
					"      x-authz-public: false\n      x-authz-because: the login page", 1)
			},
			want: "Remove the key instead",
		},
		"a misspelled extension, which would read as no mapping at all": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.manage", "      x-authz-actoin: settings.manage", 1)
			},
			want: "x-authz-actoin",
		},
		"an action internal/authz does not define": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.read", "      x-authz-action: settings.peek", 1)
			},
			want: `the action "settings.peek", which internal/authz does not define`,
		},
		"a resource that is not the one the action acts on": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.read\n"+
					"      x-authz-resource:\n        type: platform",
					"      x-authz-action: settings.read\n"+
						"      x-authz-resource:\n        type: engagement\n        engagement: engagementId", 1)
			},
			want: "settings.read acts on a platform",
		},
		"a key inside x-authz-resource that nothing reads": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-resource:\n        type: platform",
					"      x-authz-resource:\n        type: platform\n        from: path", 1)
			},
			want: "x-authz-resource.from",
		},
		"a permission on an operation that says it takes no credential": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.read\n",
					"      security: []\n      x-authz-action: settings.read\n", 1)
			},
			want: "no credential has no subject to authorize",
		},
		"both an action and an exemption": {
			break_: func(spec string) string {
				return strings.Replace(spec, "      x-authz-action: settings.read",
					"      x-authz-self: true\n      x-authz-because: nonsense\n      x-authz-action: settings.read", 1)
			},
			want: "names an action and also claims an exemption",
		},
	}

	original := string(specYAML)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broken := tc.break_(original)
			if broken == original {
				t.Fatal("this mutation no longer changes the spec — the text it targets has moved, so the rule is " +
					"untested until the mutation is updated")
			}

			doc, err := load([]byte(broken))
			if err != nil {
				t.Fatalf("the mutation broke the document itself rather than its authorization mapping: %v", err)
			}
			_, err = Requirements(doc)
			if err == nil {
				t.Fatal("Requirements accepted it. Absence, or a mistake, must never mean open")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q\nwant it to mention %q, so the reader knows what to fix", err, tc.want)
			}
		})
	}
}

// engagementSpec is a fixture for the shapes the real document has none of yet:
// an engagement-scoped resource, and a self-exempt operation with a path
// parameter. M3 adds the first for real; the second must never exist, which is
// what the test below asserts.
const engagementSpec = `
openapi: 3.1.0
info: {title: authz fixture, version: 1.0.0}
servers: [{url: /api/v1}]
paths:
  /engagements/{engagementId}:
    parameters:
      - {name: engagementId, in: path, required: true, schema: {type: string}}
    get:
      operationId: getEngagement
      summary: Read one engagement.
      x-authz-action: engagement.read
      x-authz-resource: {type: engagement, engagement: engagementId}
      responses: {"200": {description: ok}}
    delete:
      operationId: deleteEngagement
      summary: Delete one engagement.
      x-authz-action: engagement.delete
      x-authz-resource: {type: engagement, engagement: engagementId}
      responses: {"204": {description: gone}}
`

// TestAnEngagementScopedOperationSaysWhichEngagement covers the mapping shape M3
// will use, before M3 exists — the rule that catches it is the one that decides
// whether a membership is needed at all, so it is worth having a test that does
// not wait for an endpoint.
func TestAnEngagementScopedOperationSaysWhichEngagement(t *testing.T) {
	requirements, err := Requirements(fixture(t, engagementSpec))
	if err != nil {
		t.Fatalf("Requirements rejected a well-formed engagement mapping: %v", err)
	}

	requirement, ok := requirements["getEngagement"]
	if !ok {
		t.Fatal("getEngagement is missing from the mapping")
	}
	if got, want := requirement.Action, authz.ActionEngagementRead; got != want {
		t.Errorf("action = %v, want %v", got, want)
	}
	if got, want := requirement.Resource.Engagement, "engagementId"; got != want {
		t.Errorf("engagement parameter = %q, want %q", got, want)
	}
	if !requirement.Resource.Type.EngagementScoped() {
		t.Errorf("resource type %q is not engagement-scoped, so no membership would be required",
			requirement.Resource.Type)
	}
}

func TestAnEngagementScopedOperationThatNamesNoEngagementIsRefused(t *testing.T) {
	spec := strings.ReplaceAll(engagementSpec, ", engagement: engagementId}", "}")

	if _, err := Requirements(fixture(t, spec)); err == nil {
		t.Fatal("Requirements accepted an engagement resource with no engagement; every request to it would be " +
			"decided without a membership")
	} else if !strings.Contains(err.Error(), "names no engagement path parameter") {
		t.Errorf("error = %q, want it to name the missing parameter", err)
	}
}

func TestAResourceReadFromAParameterTheOperationDoesNotHaveIsRefused(t *testing.T) {
	spec := strings.ReplaceAll(engagementSpec, "engagement: engagementId}", "engagement: engagement_id}")

	if _, err := Requirements(fixture(t, spec)); err == nil {
		t.Fatal("Requirements accepted a mapping that reads a path parameter the operation does not declare")
	} else if !strings.Contains(err.Error(), `"engagement_id"`) {
		t.Errorf("error = %q, want it to name the parameter that is not there", err)
	}
}

// TestASelfOperationCannotNameAnotherResource is what keeps x-authz-self from
// becoming the hole it would otherwise be. "Acts on the caller's own account"
// holds only while the operation has no way to name anybody else's, and a path
// parameter is that way — so `/users/{userId}` cannot claim it however
// confidently the description is worded.
func TestASelfOperationCannotNameAnotherResource(t *testing.T) {
	spec := strings.Replace(engagementSpec,
		"      x-authz-action: engagement.read\n      x-authz-resource: {type: engagement, engagement: engagementId}",
		"      x-authz-self: true\n      x-authz-because: it is mine, honestly", 1)

	if _, err := Requirements(fixture(t, spec)); err == nil {
		t.Fatal("Requirements accepted x-authz-self on an operation with a path parameter; anybody signed in " +
			"could then reach anybody else's resource")
	} else if !strings.Contains(err.Error(), "engagementId") {
		t.Errorf("error = %q, want it to name the parameter that makes the claim untrue", err)
	}
}

// fixture parses one of the inline documents above. They are not run through
// [load], which enforces conventions this package's real spec follows and these
// small documents deliberately do not — the subject here is the authorization
// mapping, not the document's house style.
func fixture(t *testing.T, spec string) *openapi3.T {
	t.Helper()

	loader := &openapi3.Loader{}
	doc, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("the fixture is not a valid OpenAPI document: %v", err)
	}
	return doc
}
