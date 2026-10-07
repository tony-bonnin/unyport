package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"unyport/config"
)

const (
	csrfCookieName = "unyport_csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfMaxAge     = 3600
)

type csrfSkipKey struct{}

type csrfManager struct {
	secret []byte
	secure bool
}

var activeCSRF atomic.Pointer[csrfManager]

var bypassPaths = map[string]struct{}{
	"/api/login":          {},
	"/api/oauth/login":    {},
	"/api/oauth/callback": {},
	"/api/session":        {},
	"/api/logout":         {},
	"/sse/system":         {},
	// Ne pas bypass /api/csrf : le handler doit poser le cookie signé.
	// Logout doit rester disponible même si le token CSRF est expiré ou
	// désynchronisé côté SPA. L'action ne fait que supprimer le cookie d'auth.
}

// CSRFBypass marque les routes exclues avant que CSRFProtect les vérifie.
func CSRFBypass(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, skip := bypassPaths[r.URL.Path]; skip {
			r = r.WithContext(context.WithValue(r.Context(), csrfSkipKey{}, true))
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFProtect initialise la protection CSRF locale à partir des settings.
// Le cookie est signé HMAC, HttpOnly, SameSite=Lax, et le token doit être renvoyé
// dans X-CSRF-Token pour les méthodes dangereuses.
func CSRFProtect(s *config.Settings, trustedOrigins []string) func(http.Handler) http.Handler {
	secret, err := base64.StdEncoding.DecodeString(s.Security.CSRFSecret)
	if err != nil || len(secret) != 32 {
		panic("csrf_secret invalide (base64 de 32 bytes requis)")
	}
	manager := &csrfManager{secret: append([]byte(nil), secret...), secure: s.Security2.HTTPS}
	activeCSRF.Store(manager)
	allowedOrigins := originSet(trustedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validOrigin(r, allowedOrigins, s.Security2.HTTPS) {
				slog.Warn("CSRF origin rejected",
					"path", r.URL.Path,
					"method", r.Method,
					"host", r.Host,
					"origin", r.Header.Get("Origin"),
				)
				writeCSRFError(w, http.StatusForbidden, "csrf_origin_forbidden")
				return
			}
			if isSafeMethod(r.Method) || csrfSkipped(r) {
				next.ServeHTTP(w, r)
				return
			}
			if reason := manager.validateRequest(r); reason != "" {
				slog.Warn("CSRF invalid",
					"path", r.URL.Path,
					"method", r.Method,
					"host", r.Host,
					"origin", r.Header.Get("Origin"),
					"reason", reason,
				)
				writeCSRFError(w, http.StatusForbidden, "csrf_invalid")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfSkipped(r *http.Request) bool {
	skip, _ := r.Context().Value(csrfSkipKey{}).(bool)
	return skip
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func (m *csrfManager) validateRequest(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return "missing_cookie"
	}
	cookieToken, ok := m.parseCookie(cookie.Value)
	if !ok {
		return "invalid_cookie"
	}
	requestToken := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if requestToken == "" {
		requestToken = strings.TrimSpace(r.Header.Get("X-XSRF-Token"))
	}
	if requestToken == "" {
		return "missing_token"
	}
	if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(requestToken)) != 1 {
		return "token_mismatch"
	}
	return ""
}

func (m *csrfManager) ensureToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil {
		if token, ok := m.parseCookie(cookie.Value); ok {
			return token, nil
		}
	}
	token, value, err := m.newToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   csrfMaxAge,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return token, nil
}

func (m *csrfManager) newToken() (token string, cookieValue string, err error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(nonce)
	signature := m.sign(token)
	return token, token + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (m *csrfManager) parseCookie(value string) (string, bool) {
	token, rawSig, ok := strings.Cut(value, ".")
	if !ok || token == "" || rawSig == "" {
		return "", false
	}
	nonce, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(nonce) != 32 {
		return "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(rawSig)
	if err != nil {
		return "", false
	}
	expected := m.sign(token)
	if len(sig) != len(expected) || subtle.ConstantTimeCompare(sig, expected) != 1 {
		return "", false
	}
	return token, true
}

func (m *csrfManager) sign(token string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil)
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
	if isSafeMethod(r.Method) {
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

func writeCSRFError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// CSRFTokenHandler retourne le token CSRF courant en JSON.
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	manager := activeCSRF.Load()
	if manager == nil {
		writeCSRFError(w, http.StatusInternalServerError, "csrf_unavailable")
		return
	}
	token, err := manager.ensureToken(w, r)
	if err != nil {
		slog.Error("CSRF token generation failed", "err", err)
		writeCSRFError(w, http.StatusInternalServerError, "csrf_unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"csrf_token": token})
}
