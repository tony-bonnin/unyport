package middleware

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

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
	allowedOrigins := originSet(trustedOrigins)
	protect := csrf.Protect(
		secret,
		csrf.Secure(s.Security2.HTTPS),
		csrf.HttpOnly(true),
		csrf.CookieName("unyport_csrf_token"),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.MaxAge(3600),
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
			if !validOrigin(r, allowedOrigins, s.Security2.HTTPS) {
				slog.Warn("CSRF origin rejected",
					"path", r.URL.Path,
					"method", r.Method,
					"host", r.Host,
					"origin", r.Header.Get("Origin"),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"csrf_origin_forbidden"}`))
				return
			}
			if !s.Security2.HTTPS {
				r = csrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}
}

func originSet(origins []string) map[string]struct{} {
	set := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		set[u.Scheme+"://"+u.Host] = struct{}{}
	}
	return set
}

func validOrigin(r *http.Request, trusted map[string]struct{}, https bool) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	if _, ok := trusted[u.Scheme+"://"+u.Host]; ok {
		return true
	}

	return u.Scheme == requestScheme(r, https) && u.Host == r.Host
}

func requestScheme(r *http.Request, https bool) string {
	if https || r.TLS != nil {
		return "https"
	}
	forwarded := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if forwarded == "https" {
		return "https"
	}
	return "http"
}

// CSRFTokenHandler retourne le token CSRF courant en JSON.
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"csrf_token":"` + csrf.Token(r) + `"}`))
}
