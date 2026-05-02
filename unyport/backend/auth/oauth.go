package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/gitlab"
)

type OAuthService struct {
	providers map[string]*oauth2.Config
	users     *UserStore
	jwt       *JWTService
}

func NewOAuthService(cfg map[string]map[string]string, users *UserStore, jwt *JWTService) *OAuthService {
	providers := make(map[string]*oauth2.Config)

	if gh, ok := cfg["github"]; ok {
		providers["github"] = &oauth2.Config{
			ClientID:     gh["client_id"],
			ClientSecret: gh["client_secret"],
			RedirectURL:  gh["redirect_url"],
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
	}
	if gl, ok := cfg["gitlab"]; ok {
		providers["gitlab"] = &oauth2.Config{
			ClientID:     gl["client_id"],
			ClientSecret: gl["client_secret"],
			RedirectURL:  gl["redirect_url"],
			Scopes:       []string{"read_user"},
			Endpoint:     gitlab.Endpoint,
		}
	}
	return &OAuthService{providers: providers, users: users, jwt: jwt}
}

func (o *OAuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	conf, ok := o.providers[provider]
	if !ok {
		http.Error(w, "provider inconnu", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, conf.AuthCodeURL("random-state", oauth2.AccessTypeOffline), http.StatusFound)
}

func (o *OAuthService) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	code := r.URL.Query().Get("code")

	conf, ok := o.providers[provider]
	if !ok || code == "" {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "exchange failed", http.StatusUnauthorized)
		return
	}

	email, err := o.fetchEmail(provider, conf, token)
	if err != nil || email == "" {
		http.Error(w, "email introuvable", http.StatusUnauthorized)
		return
	}

	if _, err := o.users.Find(email); err != nil {
		_ = o.users.Add(&User{
			ID:        email,
			Email:     email,
			Roles:     []string{"user"},
			CreatedAt: time.Now(),
		})
	}

	_ = o.jwt.SetCookie(w, email)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (o *OAuthService) fetchEmail(provider string, conf *oauth2.Config, token *oauth2.Token) (string, error) {
	client := conf.Client(context.Background(), token)

	switch provider {
	case "github":
		resp, err := client.Get("https://api.github.com/user/emails")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&emails)
		for _, e := range emails {
			if e.Primary {
				return e.Email, nil
			}
		}

	case "gitlab":
		resp, err := client.Get("https://gitlab.com/api/v4/user")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		var profile struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&profile)
		return profile.Email, nil
	}

	return "", errors.New("provider non supporté")
}