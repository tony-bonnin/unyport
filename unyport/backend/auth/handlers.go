package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"unyport/config"
)

const maxJSONBodyBytes = 1 << 20
const dummyBcryptHash = "$2a$12$7FsJ23vGtaZs8705YHgEvuBP9dbp4sv8PXW7q8UcLqignMnkqrrrO"

type Handler struct {
	users  *UserStore
	jwt    *JWTService
	mailer *Mailer
	logger *slog.Logger
}

func NewHandler(users *UserStore, jwt *JWTService, mailer *Mailer, logger *slog.Logger) *Handler {
	return &Handler{users: users, jwt: jwt, mailer: mailer, logger: logger}
}

// ── Auth de base ─────────────────────────────────────────────────────────────

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &creds); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	creds.Email = strings.TrimSpace(strings.ToLower(creds.Email))

	if creds.Email == "" || creds.Password == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.users.Find(creds.Email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(creds.Password))
		h.logger.Warn("login failed: user not found", "email", creds.Email)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
		h.logger.Warn("login failed: wrong password", "email", creds.Email)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.jwt.SetCookie(w, user.Email, user.Role()); err != nil {
		h.logger.Error("jwt issue failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)
	now := time.Now()
	h.logger.Info("login ok", "email", user.Email, "role", user.Role(), "ip", ip)
	if h.mailer != nil {
		h.mailer.SendLoginNotification(user.Email, user.EffectiveDisplayName(), ip, now)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                  true,
		"email":               user.Email,
		"role":                user.Role(),
		"default_credentials": user.UsesDefaultCredentials(),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.jwt.ClearCookie(w)
	jsonOK(w)
}

// Session vérifie le JWT et retourne les infos utilisateur complètes.
func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, err := h.jwt.FromRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false})
		return
	}

	user, err := h.users.Find(claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                  true,
		"email":               user.Email,
		"role":                user.Role(),
		"display_name":        user.EffectiveDisplayName(),
		"avatar":              user.EffectiveAvatar(),
		"ssh_key":             user.SSHKey,
		"photo_url":           user.PhotoURL,
		"default_credentials": user.UsesDefaultCredentials(),
	})
}

// ── Profil utilisateur (/api/profile) ────────────────────────────────────────

// Profile retourne le profil complet de l'utilisateur connecté.
func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	email := userEmailFromCtx(r)
	if email == "" {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.users.Find(email)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"email":               user.Email,
		"role":                user.Role(),
		"display_name":        user.EffectiveDisplayName(),
		"avatar":              user.EffectiveAvatar(),
		"ssh_key":             user.SSHKey,
		"photo_url":           user.PhotoURL,
		"created_at":          user.CreatedAt.Format("2006-01-02"),
		"default_credentials": user.UsesDefaultCredentials(),
	})
}

// UpdateProfile modifie display_name, avatar, ssh_key de l'utilisateur connecté.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	email := userEmailFromCtx(r)
	if email == "" {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Avatar      string `json:"avatar"`
		PhotoURL    string `json:"photo_url"`
		SSHKey      string `json:"ssh_key"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}

	body.DisplayName = strings.TrimSpace(body.DisplayName)
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Avatar = strings.TrimSpace(body.Avatar)
	body.PhotoURL = strings.TrimSpace(body.PhotoURL)
	body.SSHKey = strings.TrimSpace(body.SSHKey)

	user, emailChanged, err := h.users.UpdateProfile(email, body.Email, body.DisplayName, body.Avatar, body.PhotoURL, body.SSHKey)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if emailChanged {
		if err := h.jwt.SetCookie(w, user.Email, user.Role()); err != nil {
			h.logger.Error("jwt reissue after email change failed", "err", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	h.logger.Info("profile updated", "email", user.Email, "previous_email", email, "email_changed", emailChanged)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                  true,
		"email":               user.Email,
		"display_name":        user.EffectiveDisplayName(),
		"avatar":              user.EffectiveAvatar(),
		"ssh_key":             user.SSHKey,
		"photo_url":           user.PhotoURL,
		"default_credentials": user.UsesDefaultCredentials(),
	})
}

// UpdatePassword change le mot de passe de l'utilisateur connecté.
func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	email := userEmailFromCtx(r)
	if email == "" {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.users.UpdatePassword(email, body.OldPassword, body.NewPassword); err != nil {
		h.logger.Warn("password change failed", "email", email, "err", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logger.Info("password changed", "email", email)
	jsonOK(w)
}

// ── Admin users (/api/admin/users) ───────────────────────────────────────────

// AdminUsers liste ou crée des utilisateurs (admin uniquement).
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.adminListUsers(w, r)
	case http.MethodPost:
		h.adminCreateUser(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) adminListUsers(w http.ResponseWriter, _ *http.Request) {
	list := h.users.List()
	type userView struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Avatar      string `json:"avatar"`
		CreatedAt   string `json:"created_at"`
	}
	out := make([]userView, 0, len(list))
	for _, u := range list {
		out = append(out, userView{
			Email:       u.Email,
			DisplayName: u.EffectiveDisplayName(),
			Role:        u.Role(),
			Avatar:      u.EffectiveAvatar(),
			CreatedAt:   u.CreatedAt.Format("2006-01-02"),
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Role = strings.TrimSpace(strings.ToLower(body.Role))
	if body.Email == "" || body.Password == "" {
		jsonError(w, "email et mot de passe requis", http.StatusBadRequest)
		return
	}
	if !ValidRoles[body.Role] {
		body.Role = "viewer"
	}
	u := &User{
		ID:        body.Email,
		Email:     body.Email,
		Password:  body.Password,
		Roles:     []string{body.Role},
		CreatedAt: time.Now(),
	}
	if err := h.users.Add(u); err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}
	caller := userEmailFromCtx(r)
	h.logger.Info("user created", "email", body.Email, "role", body.Role, "by", caller)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// AdminUserByEmail supprime un utilisateur (DELETE) ou change son rôle (PATCH).
// Routes :
//
//	DELETE /api/admin/users/{email}
//	PATCH  /api/admin/users/{email}  body: {"role": "operator"}
func (h *Handler) AdminUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		jsonError(w, "email manquant", http.StatusBadRequest)
		return
	}

	caller := userEmailFromCtx(r)

	switch r.Method {
	case http.MethodDelete:
		if email == caller {
			jsonError(w, "impossible de supprimer son propre compte", http.StatusBadRequest)
			return
		}
		if err := h.users.Delete(email); err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		h.logger.Info("user deleted", "email", email, "by", caller)
		jsonOK(w)

	case http.MethodPatch:
		var body struct {
			Role string `json:"role"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			jsonError(w, "bad request", http.StatusBadRequest)
			return
		}
		body.Role = strings.TrimSpace(strings.ToLower(body.Role))
		if email == caller && body.Role != "admin" {
			jsonError(w, "impossible de se rétrograder soi-même", http.StatusBadRequest)
			return
		}
		if err := h.users.SetRole(email, body.Role); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Info("role changed", "email", email, "role", body.Role, "by", caller)
		jsonOK(w)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Branding (/api/branding) ─────────────────────────────────────────────────

// BrandingHandler gère GET et PATCH /api/branding.
type BrandingHandler struct {
	store  *config.BrandingStore
	logger *slog.Logger
}

func NewBrandingHandler(store *config.BrandingStore, logger *slog.Logger) *BrandingHandler {
	return &BrandingHandler{store: store, logger: logger}
}

// GetBranding — GET /api/branding
// Public (avant auth) : le frontend en a besoin au chargement initial
// pour appliquer les couleurs et le logo avant même le login.
func (h *BrandingHandler) GetBranding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b := h.store.Get()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"logo_src": b.EffectiveLogoSrc(),
		"has_logo": b.HasCustomLogo(),
		"colors": map[string]string{
			"dom0":      b.Colors.Dom0,
			"domu":      b.Colors.DomU,
			"container": b.Colors.Container,
			"alpine":    b.Colors.Alpine,
		},
	})
}

// UpdateBranding — PATCH /api/branding/update (admin uniquement)
func (h *BrandingHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		LogoBase64 string `json:"logo_base64"`
		LogoURL    string `json:"logo_url"`
		Colors     struct {
			Dom0      string `json:"dom0"`
			DomU      string `json:"domu"`
			Container string `json:"container"`
			Alpine    string `json:"alpine"`
		} `json:"colors"`
	}

	if err := decodeJSON(w, r, &body); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}

	b := config.Branding{
		LogoBase64: body.LogoBase64,
		LogoURL:    body.LogoURL,
		Colors: config.BrandingColors{
			Dom0:      body.Colors.Dom0,
			DomU:      body.Colors.DomU,
			Container: body.Colors.Container,
			Alpine:    body.Colors.Alpine,
		},
	}

	if err := h.store.Update(b); err != nil {
		h.logger.Warn("branding update failed", "err", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	caller := userEmailFromCtx(r)
	h.logger.Info("branding updated", "by", caller)
	jsonOK(w)
}

// ResetBranding — DELETE /api/branding/reset (admin) — remet les valeurs par défaut
func (h *BrandingHandler) ResetBranding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.Update(config.Branding{}); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	caller := userEmailFromCtx(r)
	h.logger.Info("branding reset", "by", caller)
	jsonOK(w)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid json")
	}
	return nil
}

// userEmailFromCtx lit la clé "user_id" du context (posée par le middleware Auth).
func userEmailFromCtx(r *http.Request) string {
	v, _ := r.Context().Value("user_id").(string)
	return v
}

func clientIP(r *http.Request) string {
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			fwd = fwd[:i]
		}
		return strings.TrimSpace(fwd)
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return ip
	}
	return r.RemoteAddr
}
