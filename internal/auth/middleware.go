package auth

import (
	"context"
	"net/http"
	"strings"

	"recipe_manager/internal/storage"
)

type contextKey string

const userContextKey contextKey = "user"

// RequireAuth returns middleware that validates the JWT from the Authorization
// header or the "session" cookie, injects the user into the request context.
func RequireAuth(db *storage.DB, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := tokenFromRequest(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			userID, err := Verify(tokenStr, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			user, err := db.GetUser(userID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user from the request context.
func UserFromContext(ctx context.Context) *storage.User {
	u, _ := ctx.Value(userContextKey).(*storage.User)
	return u
}

// tokenFromRequest extracts the Bearer token from the Authorization header
// or falls back to the "session" cookie.
func tokenFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("session"); err == nil {
		return c.Value
	}
	return ""
}
