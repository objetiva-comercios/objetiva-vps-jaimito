package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/chiguire/jaimito/internal/db"
)

// BearerAuth returns a middleware that authenticates requests using Bearer tokens.
// Tokens must start with "sk-" and be found in the database (not revoked).
// On success, the request proceeds; on failure, 401 is returned.
func BearerAuth(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				WriteError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if !strings.HasPrefix(token, "sk-") {
				WriteError(w, http.StatusUnauthorized, "invalid token format")
				return
			}

			keyHash := db.HashToken(token)
			keyID, err := db.LookupKeyHash(r.Context(), database, keyHash)
			if err != nil || keyID == "" {
				WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			// Fire-and-forget: update last_used_at. Use background context so the
			// update is not cancelled when the request context ends.
			go db.UpdateLastUsed(context.Background(), database, keyID) //nolint:errcheck

			next.ServeHTTP(w, r)
		})
	}
}
