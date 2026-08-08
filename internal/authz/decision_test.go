package authz_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
)

// TestTheLogLineSaysWhoAskedForWhatAndWhy is M1-012's second acceptance
// criterion. A denial nobody can attribute is v1's bare 403 with extra steps:
// the operator cannot tell an intended refusal from a rule somebody forgot,
// which is why the forgotten ones survived.
func TestTheLogLineSaysWhoAskedForWhatAndWhy(t *testing.T) {
	var captured bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&captured, &slog.HandlerOptions{Level: slog.LevelDebug}))

	subject := member(authz.EngagementRoleObserver)
	resource := executionIn(engagement)
	decision := authz.Can(t.Context(), subject, authz.ActionExecutionWriteRed, resource)

	authz.Log(t.Context(), log, subject, authz.ActionExecutionWriteRed, resource, decision)

	var line struct {
		Subject struct {
			UserID       string `json:"user_id"`
			PlatformRole string `json:"platform_role"`
			Method       string `json:"method"`
			MFASatisfied bool   `json:"mfa_satisfied"`
		} `json:"subject"`
		Action   string `json:"action"`
		Resource struct {
			Type         string `json:"type"`
			ID           string `json:"id"`
			EngagementID string `json:"engagement_id"`
		} `json:"resource"`
		Decision struct {
			Allowed bool   `json:"allowed"`
			Reason  string `json:"reason"`
		} `json:"decision"`
	}
	if err := json.Unmarshal(captured.Bytes(), &line); err != nil {
		t.Fatalf("decoding the log line: %v\n%s", err, captured.String())
	}

	switch {
	case line.Subject.UserID != subject.UserID:
		t.Errorf("subject.user_id = %q, want %q", line.Subject.UserID, subject.UserID)
	case line.Subject.PlatformRole != string(subject.PlatformRole):
		t.Errorf("subject.platform_role = %q, want %q", line.Subject.PlatformRole, subject.PlatformRole)
	case line.Subject.Method != string(authz.MethodCookie):
		t.Errorf("subject.method = %q, want %q", line.Subject.Method, authz.MethodCookie)
	case line.Action != "execution.write_red":
		t.Errorf("action = %q, want %q", line.Action, "execution.write_red")
	case line.Resource.Type != string(authz.ResourceExecution):
		t.Errorf("resource.type = %q, want %q", line.Resource.Type, authz.ResourceExecution)
	case line.Resource.EngagementID != engagement:
		t.Errorf("resource.engagement_id = %q, want %q", line.Resource.EngagementID, engagement)
	case line.Decision.Allowed:
		t.Error("decision.allowed is true for a refused observer")
	case line.Decision.Reason != decision.Reason:
		t.Errorf("decision.reason = %q, want %q", line.Decision.Reason, decision.Reason)
	}
}

// TestALogLineNeverCarriesAToken. A Subject has never held the secret — only
// what resolving it produced — so there is nothing here to leak. M1-011
// requires a token's secret to appear in exactly one response ever and in no
// log; this asserts the half of that which this package is responsible for.
func TestALogLineNeverCarriesAToken(t *testing.T) {
	const secret = "bl_abcd1234_thisisthesecretpart"

	var captured bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&captured, &slog.HandlerOptions{Level: slog.LevelDebug}))

	subject := admin()
	subject.Method = authz.MethodServiceToken
	subject.TokenScopes = []authz.TokenScope{authz.TokenScopeAdminRead}
	resource := authz.Resource{Type: authz.ResourceUser, ID: "user-2"}

	authz.Log(t.Context(), log, subject, authz.ActionUserRead, resource,
		authz.Can(t.Context(), subject, authz.ActionUserRead, resource))

	logged := captured.String()
	if strings.Contains(logged, secret) {
		t.Fatalf("the log line contains a token secret: %s", logged)
	}
	if !strings.Contains(logged, string(authz.TokenScopeAdminRead)) {
		t.Errorf("the log line omits the token's scopes, which is what makes a token decision readable: %s", logged)
	}
}

// TestLogWithoutALoggerIsNotACrash. The middleware constructs one; a test, a
// command-line tool or an early-startup path might not, and a policy call that
// panicked for want of somewhere to write would be an outage caused by logging.
func TestLogWithoutALoggerIsNotACrash(t *testing.T) {
	authz.Log(t.Context(), nil, admin(), authz.ActionUserRead,
		authz.Resource{Type: authz.ResourceUser}, authz.Decision{})
}

// TestTheZeroDecisionIsADenial. Nothing here returns one, but a caller that
// declares `var decision authz.Decision` and forgets to assign it must fail
// closed.
func TestTheZeroDecisionIsADenial(t *testing.T) {
	var decision authz.Decision

	if decision.Allowed {
		t.Fatal("the zero Decision allows")
	}
}
