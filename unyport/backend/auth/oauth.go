package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/gitlab"
)

type OAuthService struct {
	providers map[string]*oauth2.Config
	users     *UserStore
	jwt       *JWTService
	mailer    *Mailer
	secure    bool
}

func NewOAuthService(cfg map[string]map[string]string, users *UserStore, jwt *JWTService, mailer *Mailer, secure bool) *OAuthService {
	providers := make(map[string]*oauth2.Config)

	if gh, ok := cfg["github"]; ok && oauthProviderConfigured(gh) {
		providers["github"] = &oauth2.Config{
			ClientID:     gh["client_id"],
			ClientSecret: gh["client_secret"],
			RedirectURL:  gh["redirect_url"],
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
	}
	if gl, ok := cfg["gitlab"]; ok && oauthProviderConfigured(gl) {
		providers["gitlab"] = &oauth2.Config{
			ClientID:     gl["client_id"],
			ClientSecret: gl["client_secret"],
			RedirectURL:  gl["redirect_url"],
			Scopes:       []string{"read_user"},
			Endpoint:     gitlab.Endpoint,
		}
	}
	return &OAuthService{providers: providers, users: users, jwt: jwt, mailer: mailer, secure: secure}
}

func (o *OAuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	provider := r.URL.Query().Get("provider")
	conf, ok := o.providers[provider]
	if !ok {
		http.Error(w, "provider inconnu", http.StatusBadRequest)
		return
	}
	state, err := randomState()
	if err != nil {
		http.Error(w, "state failed", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "unyport_oauth_state",
		Value:    state,
		Path:     "/api/oauth/callback",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   o.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, conf.AuthCodeURL(state), http.StatusFound)
}

func (o *OAuthService) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	provider := r.URL.Query().Get("provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	conf, ok := o.providers[provider]
	if !ok || code == "" {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}
	stateCookie, err := r.Cookie("unyport_oauth_state")
	if err != nil || state == "" || stateCookie.Value != state {
		http.Error(w, "state invalide", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "unyport_oauth_state",
		Value:    "",
		Path:     "/api/oauth/callback",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   o.secure,
		SameSite: http.SameSiteLaxMode,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "exchange failed", http.StatusUnauthorized)
		return
	}

	email, err := o.fetchEmail(provider, conf, token)
	if err != nil || email == "" {
		http.Error(w, "email introuvable", http.StatusUnauthorized)
		return
	}
	email = strings.ToLower(strings.TrimSpace(email))

	if _, err := o.users.Find(email); err != nil {
		_ = o.users.Add(&User{
			ID:        email,
			Email:     email,
			Roles:     []string{"viewer"},
			CreatedAt: time.Now(),
		})
	}

	u, err := o.users.Find(email)
	if err != nil {
		http.Error(w, "user failed", http.StatusUnauthorized)
		return
	}
	if err := o.jwt.SetCookie(w, email, u.Role()); err != nil {
		http.Error(w, "token failed", http.StatusInternalServerError)
		return
	}
	if o.mailer != nil {
		o.mailer.SendLoginNotification(u.Email, u.EffectiveDisplayName(), clientIP(r), time.Now())
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (o *OAuthService) fetchEmail(provider string, conf *oauth2.Config, token *oauth2.Token) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := conf.Client(ctx, token)

	switch provider {
	case "github":
		resp, err := client.Get("https://api.github.com/user/emails")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", errors.New("github user emails failed")
		}
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
			return "", err
		}
		for _, e := range emails {
			if e.Primary && e.Verified {
				return e.Email, nil
			}
		}

	case "gitlab":
		resp, err := client.Get("https://gitlab.com/api/v4/user")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", errors.New("gitlab user failed")
		}
		var profile struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
			return "", err
		}
		return profile.Email, nil
	}

	return "", errors.New("provider non supporté")
}

func randomState() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func oauthProviderConfigured(conf map[string]string) bool {
	for _, key := range []string{"client_id", "client_secret", "redirect_url"} {
		value := strings.TrimSpace(conf[key])
		if value == "" || strings.Contains(value, "abcdef") || strings.Contains(value, "supersecret") || strings.Contains(value, "mon-domaine.com") {
			return false
		}
	}
	return true
}
