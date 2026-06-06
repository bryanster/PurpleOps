package handler

import (
	"net/http"
	"net/url"
	"testing"
)

// --- HandleCreateUser ---

func TestHandleCreateUser_MissingEmail(t *testing.T) {
	form := url.Values{
		"username": {"testuser"},
		"password": {"password123"},
	}
	c, w := newGinContext("POST", "/manage/access/user", form, nil, nil)
	HandleCreateUser(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing email: expected 400, got %d", w.Code)
	}
}

func TestHandleCreateUser_MissingUsername(t *testing.T) {
	form := url.Values{
		"email":    {"user@example.com"},
		"password": {"password123"},
	}
	c, w := newGinContext("POST", "/manage/access/user", form, nil, nil)
	HandleCreateUser(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing username: expected 400, got %d", w.Code)
	}
}

func TestHandleCreateUser_MissingPassword(t *testing.T) {
	form := url.Values{
		"email":    {"user@example.com"},
		"username": {"testuser"},
	}
	c, w := newGinContext("POST", "/manage/access/user", form, nil, nil)
	HandleCreateUser(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing password: expected 400, got %d", w.Code)
	}
}

func TestHandleCreateUser_AllEmpty(t *testing.T) {
	form := url.Values{}
	c, w := newGinContext("POST", "/manage/access/user", form, nil, nil)
	HandleCreateUser(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("all empty: expected 400, got %d", w.Code)
	}
}
