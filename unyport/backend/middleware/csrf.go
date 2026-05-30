package middleware

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/gorilla/csrf"
	"unyport/config"
)

var bypassPaths = map[string]struct{}{
	"/api/login":          {},
	"/api/oauth/login":    {},
	"/api/oauth/callback": {},
	"/api/session":        {},
	"/api/logout":         {},
	"/sse/system":         {},
	// Ne pas bypass /api/csrf : gorilla/csrf doit traverser cette route pour
	// générer le token masqué et poser le cookie signé correspondant.
	// Logout doit rester disponible même si le token CSRF est expiré ou
	// désynchronisé côté SPA. L'action ne fait que supprimer le cookie d'auth.
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
// Le flag Secure est piloté par settings.Security2.HTTPS.
func CSRFProtect(s *config.Settings, trustedOrigins []string) func(http.Handler) http.Handler {
	secret, err := base64.StdEncoding.DecodeString(s.Security.CSRFSecret)
	if err != nil || len(secret) != 32 {
		panic("csrf_secret invalide (base64 de 32 bytes requis)")
	}
	protect := csrf.Protect(
		secret,
		csrf.Secure(s.Security2.HTTPS),
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
				"host", r.Host,
				"origin", r.Header.Get("Origin"),
				"reason", csrf.FailureReason(r),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"csrf_invalid"}`))
		})),
	)
	return func(next http.Handler) http.Handler {
		protected := protect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.Security2.HTTPS {
				r = csrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}
}

// CSRFTokenHandler retourne le token CSRF courant en JSON.
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"csrf_token":"` + csrf.Token(r) + `"}`))
}
