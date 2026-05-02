package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	users  *UserStore
	jwt    *JWTService
	logger *slog.Logger
}

func NewHandler(users *UserStore, jwt *JWTService, logger *slog.Logger) *Handler {
	return &Handler{users: users, jwt: jwt, logger: logger}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, err := h.users.Find(creds.Email)
	if err != nil {
		h.logger.Warn("login failed: user not found", "email", creds.Email)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
		h.logger.Warn("login failed: wrong password", "email", creds.Email)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.jwt.SetCookie(w, user.Email); err != nil {
		h.logger.Error("jwt issue failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("login ok", "email", user.Email)
	jsonOK(w)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.jwt.ClearCookie(w)
	jsonOK(w)
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	if _, err := h.jwt.FromRequest(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jsonOK(w)
}

func jsonOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}