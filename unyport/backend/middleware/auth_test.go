package middleware

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"unyport/auth"
)

func TestAuthUsesCurrentStoreRoleInsteadOfJWTClaim(t *testing.T) {
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.json")
	usersJSON := `{
  "demo@example.com": {
    "id": "demo@example.com",
    "email": "demo@example.com",
    "password": "$2a$12$7FsJ23vGtaZs8705YHgEvuBP9dbp4sv8PXW7q8UcLqignMnkqrrrO",
    "roles": ["viewer"],
    "created_at": "2026-05-29T00:00:00Z"
  }
}`
	if err := os.WriteFile(usersPath, []byte(usersJSON), 0600); err != nil {
		t.Fatal(err)
	}

	users, err := auth.NewUserStore(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	secret := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	jwtSvc, err := auth.NewJWTServiceWithOpts(secret, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}

	cookieRecorder := httptest.NewRecorder()
	if err := jwtSvc.SetCookie(cookieRecorder, "demo@example.com", "admin"); err != nil {
		t.Fatal(err)
	}
	cookies := cookieRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected auth cookie, got %d", len(cookies))
	}

	protected := Auth(jwtSvc, users, slog.New(slog.NewTextHandler(io.Discard, nil)))(
		RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected current viewer role to be forbidden, got %d", rec.Code)
	}
}
