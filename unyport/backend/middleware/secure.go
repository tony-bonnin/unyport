package middleware

import (
	"net/http"
	"strings"
)

// Secure injecte les headers de sécurité HTTP.
// Les routes /proxy/* reçoivent une politique allégée (CSP gérée par le proxy).
func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if !strings.HasPrefix(r.URL.Path, "/proxy/") {
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
				"base-uri 'self'; "+
				"frame-ancestors 'none'; "+
				"object-src 'none'; "+
				"img-src 'self';"+
				"connect-src 'self' ws: wss:; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"font-src 'self'; ")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		next.ServeHTTP(w, r)
	})
}