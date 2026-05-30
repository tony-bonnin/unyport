package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"unyport/auth"
)

const (
	UserIDKey   = "user_id"
	UserRoleKey = "user_role"
)

// Auth protège les routes : vérifie le JWT, redirige vers / ou renvoie 401.
func Auth(jwt *auth.JWTService, users *auth.UserStore, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := jwt.FromRequest(r)
			if err != nil {
				logger.Debug("auth failed", "path", r.URL.Path, "err", err)
				if isSSERequest(r) {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if isHTMLRequest(r) {
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
				return
			}

			user, err := users.Find(claims.UserID)
			if err != nil {
				logger.Warn("auth failed: user no longer exists", "user", claims.UserID, "path", r.URL.Path)
				jwt.ClearCookie(w)
				if isSSERequest(r) {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if isHTMLRequest(r) {
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, user.Email)
			ctx = context.WithValue(ctx, UserRoleKey, user.Role())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole refuse les requêtes dont le rôle JWT n'est pas dans la liste autorisée.
// Doit être appliqué APRÈS Auth (UserRoleKey doit être dans le context).
// Ordre de permissivité : admin > operator > viewer.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(UserRoleKey).(string)
			if !allowed[role] {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserEmailFromCtx extrait l'email utilisateur du context (mis par Auth).
func UserEmailFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// UserRoleFromCtx extrait le rôle utilisateur du context (mis par Auth).
func UserRoleFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(UserRoleKey).(string)
	return v
}

func isSSERequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/sse/")
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
