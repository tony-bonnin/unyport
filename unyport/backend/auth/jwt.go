package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const cookieName = "unyport_auth"
const tokenTTL = time.Hour

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

type JWTService struct {
	key []byte
}

func NewJWTService(b64secret string) (*JWTService, error) {
	if b64secret == "" {
		return nil, errors.New("jwt_secret manquant dans settings.yaml")
	}
	key, err := base64.StdEncoding.DecodeString(b64secret)
	if err != nil {
		return nil, errors.New("jwt_secret: base64 invalide")
	}
	if len(key) < 32 {
		return nil, errors.New("jwt_secret trop court (minimum 32 bytes)")
	}
	return &JWTService{key: key}, nil
}

func (j *JWTService) Issue(userID string) (string, time.Time, error) {
	exp := time.Now().Add(tokenTTL)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.key)
	return tok, exp, err
}

func (j *JWTService) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.key, nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("token invalide ou expiré")
	}
	return claims, nil
}

func (j *JWTService) SetCookie(w http.ResponseWriter, userID string) error {
	tok, exp, err := j.Issue(userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Expires:  exp,
		HttpOnly: true,
		Secure:   false, // true derrière TLS
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	return nil
}

func (j *JWTService) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (j *JWTService) FromRequest(r *http.Request) (*Claims, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, errors.New("cookie absent")
	}
	return j.Parse(c.Value)
}