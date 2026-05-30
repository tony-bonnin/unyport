package middleware

import (
	"net/http"
	"strings"
)

// Secure injecte les headers de sécurité HTTP.
// Les routes /proxy/* reçoivent une politique allégée (CSP gérée par le proxy).
func Secure(next http.Handler, https bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		if !strings.HasPrefix(r.URL.Path, "/proxy/") {
			// Chaque directive séparée par "; " — pas de concaténation partielle.
			// img-src : 'self' + data: pour les logos base64 éventuels.
			h.Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'self'",
				"base-uri 'self'",
				"frame-ancestors 'none'",
				"object-src 'none'",
				"img-src 'self' data:",
				"connect-src 'self' ws: wss:",
				"script-src 'self'",
				"style-src 'self'",
				"font-src 'self'",
				"form-action 'self'",
			}, "; "))

			if https {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
		}

		next.ServeHTTP(w, r)
	})
}
