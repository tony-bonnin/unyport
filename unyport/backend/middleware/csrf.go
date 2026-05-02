package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/csrf"
	"unyport/config"
)

var bypassPaths = map[string]struct{}{
	"/api/login":          {},
	"/api/logout":         {},
	"/api/oauth/login":    {},
	"/api/oauth/callback": {},
	"/api/csrf":           {},
	"/api/session":        {},
	"/sse/system":         {},
}

// CSRFBypass marque les routes exclues avant que CSRFProtect les vérifie.
func CSRFBypass(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, skip := bypassPaths[r.URL.Path]; skip {
			r = csrf.UnsafeSkipCheck(r)
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFProtect initialise gorilla/csrf à partir des settings.
func CSRFProtect(s *config.Settings, trustedOrigins []string) func(http.Handler) http.Handler {
	secret := []byte(s.Security.CSRFSecret)
	if len(secret) < 32 {
		panic("csrf_secret trop court (minimum 32 bytes)")
	}
	return csrf.Protect(
		secret,
		csrf.Secure(false), // true en prod HTTPS
		csrf.HttpOnly(true),
		csrf.CookieName("unyport_csrf_token"),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.MaxAge(3600),
		csrf.TrustedOrigins(trustedOrigins),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("CSRF invalid",
				"path", r.URL.Path,
				"method", r.Method,
				"origin", r.Header.Get("Origin"),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"csrf_invalid"}`))
		})),
	)
}

// CSRFTokenHandler retourne le token CSRF courant en JSON.
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"csrf_token":"` + csrf.Token(r) + `"}`))
}