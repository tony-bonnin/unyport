package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecureCSPForbidsInlineStylesAndScripts(t *testing.T) {
	handler := Secure(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"script-src 'self'",
		"style-src 'self'",
		"style-src-elem 'self'",
		"style-src-attr 'none'",
	} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("CSP missing %q: %s", directive, csp)
		}
	}
	if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("CSP contains an unsafe execution mode: %s", csp)
	}
}
