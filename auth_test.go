package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "securepassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == password {
		t.Error("hash should not equal plain password")
	}

	if !CheckPassword(hash, password) {
		t.Error("CheckPassword should return true for correct password")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestCheckPasswordInvalidHash(t *testing.T) {
	if CheckPassword("not-a-valid-hash", "password") {
		t.Error("CheckPassword should return false for invalid hash")
	}
}

func TestUserFromContext(t *testing.T) {
	// Empty context
	ctx := context.Background()
	if user := UserFromContext(ctx); user != nil {
		t.Error("UserFromContext should return nil for empty context")
	}

	// Context with user
	u := &User{ID: bson.NewObjectID(), Username: "testuser"}
	ctx = context.WithValue(context.Background(), userContextKey, u)
	if user := UserFromContext(ctx); user == nil {
		t.Error("UserFromContext should return user from context")
	} else if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}

	// Context with wrong type
	ctx = context.WithValue(context.Background(), userContextKey, "not-a-user")
	if user := UserFromContext(ctx); user != nil {
		t.Error("UserFromContext should return nil for wrong type")
	}
}

func TestExtractID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/assessment/abc123/", "abc123"},
		{"/assessment/abc123", "abc123"},
		{"/testcase/def456/", "def456"},
		{"/testcase/def456", "def456"},
		{"/assessment/abc123/navigator", "abc123"},
		{"/testcase/def456/clone", "def456"},
		{"/login", ""},
		{"/", ""},
		{"/assessment/", ""},
		{"/testcase/", ""},
	}

	for _, tt := range tests {
		r := httptest.NewRequest("GET", tt.path, nil)
		got := extractID(r)
		if got != tt.want {
			t.Errorf("extractID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestInitSessions(t *testing.T) {
	InitSessions("test-secret-key")

	if store == nil {
		t.Fatal("store should not be nil after InitSessions")
	}
	if store.Options.Path != "/" {
		t.Errorf("expected path '/', got %q", store.Options.Path)
	}
	if store.Options.MaxAge != 86400*7 {
		t.Errorf("expected MaxAge 604800, got %d", store.Options.MaxAge)
	}
	if !store.Options.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
}

func TestSetAndClearSession(t *testing.T) {
	InitSessions("test-secret-key")

	// Create a request/response pair
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set session user
	SetSessionUser(w, r, "user123")

	sess := GetSession(r)
	if sess.Values["user_id"] != "user123" {
		t.Errorf("expected user_id 'user123', got %v", sess.Values["user_id"])
	}

	// Clear session
	ClearSession(w, r)
	sess = GetSession(r)
	if _, ok := sess.Values["user_id"]; ok {
		t.Error("expected user_id to be cleared")
	}
}

func TestAuthRequiredMiddleware(t *testing.T) {
	InitSessions("test-secret-key")

	handler := AuthRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without auth - should redirect
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
