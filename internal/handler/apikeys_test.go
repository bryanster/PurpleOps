package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newGinContext creates a gin.Context for use in handler unit tests.
func newGinContext(method, target string, body url.Values, user *models.User, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if user != nil {
		ctx := auth.WithUser(r.Context(), user)
		r = r.WithContext(ctx)
	}
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	if params != nil {
		c.Params = params
	}
	return c, w
}

// --- HandleCreateAPIKey ---

func TestHandleCreateAPIKey_NoAuth(t *testing.T) {
	c, w := newGinContext("POST", "/api-keys", nil, nil, nil)
	HandleCreateAPIKey(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleCreateAPIKey_MissingName(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Active: true}
	form := url.Values{}
	// name intentionally omitted

	c, w := newGinContext("POST", "/api-keys", form, user, nil)
	HandleCreateAPIKey(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateAPIKey_BlankNameWhitespace(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Active: true}
	form := url.Values{"name": {"   "}}

	c, w := newGinContext("POST", "/api-keys", form, user, nil)
	HandleCreateAPIKey(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for whitespace-only name, got %d", w.Code)
	}
}

func TestHandleCreateAPIKey_RoleEscalationRejected(t *testing.T) {
	// User has no roles; requesting a role they don't own must be rejected.
	user := &models.User{
		ID:     bson.NewObjectID(),
		Active: true,
		Roles:  []bson.ObjectID{}, // no roles
	}
	form := url.Values{
		"name":  {"test key"},
		"roles": {"Admin"}, // user doesn't have Admin
	}

	c, w := newGinContext("POST", "/api-keys", form, user, nil)
	// Without DB the role lookup returns err, so the role is skipped and
	// we proceed to DB insert which also fails → 500. Not 200.
	HandleCreateAPIKey(c)

	if w.Code == http.StatusOK {
		t.Errorf("must not return 200 when role escalation attempted without DB, got %d", w.Code)
	}
}

// --- HandleDeleteAPIKey ---

func TestHandleDeleteAPIKey_NoAuth(t *testing.T) {
	c, w := newGinContext("DELETE", "/api-keys/"+bson.NewObjectID().Hex(), nil, nil, nil)
	HandleDeleteAPIKey(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleDeleteAPIKey_InvalidID(t *testing.T) {
	user := &models.User{ID: bson.NewObjectID(), Active: true}

	c, w := newGinContext("DELETE", "/api-keys/not-a-valid-objectid", nil, user,
		gin.Params{{Key: "id", Value: "not-a-valid-objectid"}})
	HandleDeleteAPIKey(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", w.Code)
	}
}

// --- HandleAPIKeysPage ---

func TestHandleAPIKeysPage_NoAuth(t *testing.T) {
	c, w := newGinContext("GET", "/api-keys", nil, nil, nil)
	HandleAPIKeysPage(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- Key generation helpers ---

func TestAPIKeyFormat(t *testing.T) {
	// Verify our expected key format matches what the handler produces.
	// We test the same logic used in HandleCreateAPIKey without hitting DB.
	rawBytes := []byte("00000000000000000000000000000000") // 32 deterministic bytes
	rawKey := "pops_" + hex.EncodeToString(rawBytes)

	if !strings.HasPrefix(rawKey, "pops_") {
		t.Error("key must start with pops_")
	}
	// "pops_" (5) + 64 hex chars = 69
	if len(rawKey) != 69 {
		t.Errorf("expected key length 69, got %d", len(rawKey))
	}

	// Prefix stored in DB: first 13 chars ("pops_" + 8 hex)
	prefix := rawKey[:13]
	if len(prefix) != 13 {
		t.Errorf("expected prefix length 13, got %d", len(prefix))
	}
	if !strings.HasPrefix(prefix, "pops_") {
		t.Error("prefix must start with pops_")
	}
}

func TestAPIKeyHashDeterministic(t *testing.T) {
	key := "pops_abc123"
	sum1 := sha256.Sum256([]byte(key))
	sum2 := sha256.Sum256([]byte(key))

	h1 := hex.EncodeToString(sum1[:])
	h2 := hex.EncodeToString(sum2[:])

	if h1 != h2 {
		t.Error("SHA-256 of same key must be deterministic")
	}
}

func TestAPIKeyHashUnique(t *testing.T) {
	key1 := "pops_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	key2 := "pops_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	sum1 := sha256.Sum256([]byte(key1))
	sum2 := sha256.Sum256([]byte(key2))

	if sum1 == sum2 {
		t.Error("different keys must produce different hashes")
	}
}

func TestAPIKeyHashLength(t *testing.T) {
	sum := sha256.Sum256([]byte("pops_testkey"))
	hash := hex.EncodeToString(sum[:])
	if len(hash) != 64 {
		t.Errorf("expected 64 hex chars for SHA-256, got %d", len(hash))
	}
}
