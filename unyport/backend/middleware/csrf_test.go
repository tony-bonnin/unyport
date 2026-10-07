package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"unyport/config"
)

func testCSRFSettings() *config.Settings {
	return &config.Settings{
		Security: struct {
			CSRFSecret string `yaml:"csrf_secret"`
			JWTSecret  string `yaml:"jwt_secret"`
		}{
			CSRFSecret: base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")),
		},
	}
}

func fetchTestCSRFToken(t *testing.T, handler http.Handler) (string, *http.Cookie) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("csrf token status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode csrf body: %v", err)
	}
	token := body["csrf_token"]
	if token == "" {
		t.Fatal("csrf token is empty")
	}
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			return token, cookie
		}
	}
	t.Fatal("csrf cookie not set")
	return "", nil
}

func TestCSRFProtectAcceptsSignedCookieAndHeader(t *testing.T) {
	settings := testCSRFSettings()
	tokenHandler := CSRFProtect(settings, nil)(http.HandlerFunc(CSRFTokenHandler))
	token, cookie := fetchTestCSRFToken(t, tokenHandler)

	protected := CSRFBypass(CSRFProtect(settings, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/api/profile", strings.NewReader(`{}`))
	req.Host = "example.test"
	req.Header.Set("Origin", "http://example.test")
	req.Header.Set(csrfHeaderName, token)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("protected status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFProtectRejectsMissingToken(t *testing.T) {
	settings := testCSRFSettings()
	tokenHandler := CSRFProtect(settings, nil)(http.HandlerFunc(CSRFTokenHandler))
	_, cookie := fetchTestCSRFToken(t, tokenHandler)

	protected := CSRFBypass(CSRFProtect(settings, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/api/profile", strings.NewReader(`{}`))
	req.Host = "example.test"
	req.Header.Set("Origin", "http://example.test")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("protected status = %d, want 403", rr.Code)
	}
}

func TestCSRFBypassStillRejectsBadOrigin(t *testing.T) {
	settings := testCSRFSettings()
	protected := CSRFBypass(CSRFProtect(settings, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{}`))
	req.Host = "example.test"
	req.Header.Set("Origin", "http://evil.test")
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("login status = %d, want 403 for bad origin", rr.Code)
	}
}
