package render

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/purpleops/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestTemplateUserIsAuthenticated(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if tu.IsAuthenticated() {
		t.Error("expected not authenticated with nil user")
	}

	tu = NewTemplateUser(&models.User{}, context.Background())
	if !tu.IsAuthenticated() {
		t.Error("expected authenticated with non-nil user")
	}
}

func TestTemplateUserGetUsername(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if got := tu.GetUsername(); got != "" {
		t.Errorf("expected empty string for nil user, got %q", got)
	}

	tu = NewTemplateUser(&models.User{Username: "testuser"}, context.Background())
	if got := tu.GetUsername(); got != "testuser" {
		t.Errorf("expected 'testuser', got %q", got)
	}
}

func TestTemplateUserGetInitpwd(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if tu.GetInitpwd() {
		t.Error("expected false for nil user")
	}

	tu = NewTemplateUser(&models.User{InitPwd: true}, context.Background())
	if !tu.GetInitpwd() {
		t.Error("expected true when InitPwd is true")
	}

	tu = NewTemplateUser(&models.User{InitPwd: false}, context.Background())
	if tu.GetInitpwd() {
		t.Error("expected false when InitPwd is false")
	}
}

func TestTemplateUserGetID(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if got := tu.GetID(); got != "" {
		t.Errorf("expected empty string for nil user, got %q", got)
	}

	id := bson.NewObjectID()
	tu = NewTemplateUser(&models.User{ID: id}, context.Background())
	if got := tu.GetID(); got != id.Hex() {
		t.Errorf("expected %q, got %q", id.Hex(), got)
	}
}

func TestTemplateUserHasRoleNilUser(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if tu.HasRole("Admin") {
		t.Error("expected false for nil user")
	}
}

func TestTemplateUserAssessmentListNilUser(t *testing.T) {
	tu := NewTemplateUser(nil, context.Background())
	if got := tu.AssessmentList(); got != nil {
		t.Errorf("expected nil for nil user, got %v", got)
	}
}

func TestTemplateRequestGetPath(t *testing.T) {
	r := httptest.NewRequest("GET", "/assessment/123", nil)
	tr := NewTemplateRequest(r)
	if got := tr.GetPath(); got != "/assessment/123" {
		t.Errorf("expected '/assessment/123', got %q", got)
	}
}
