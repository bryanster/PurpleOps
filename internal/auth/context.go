package auth

import (
	"context"

	"github.com/bryanster/purpleops/internal/models"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// userContextKey is the context key for the authenticated user.
const userContextKey contextKey = "currentUser"

// UserFromContext retrieves the authenticated user from the context, or nil.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// WithUser returns a new context carrying the given user.
func WithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}
