package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"unyport/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// Auth protège les routes : vérifie le JWT, redirige vers / ou renvoie 401.
func Auth(jwt *auth.JWTService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := jwt.FromRequest(r)
			if err != nil {
				logger.Debug("auth failed", "path", r.URL.Path, "err", err)
				if isHTMLRequest(r) {
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isHTMLRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "text/html") ||
		(!strings.Contains(accept, "application/json") &&
			strings.ToLower(r.Header.Get("X-Requested-With")) != "xmlhttprequest")
}