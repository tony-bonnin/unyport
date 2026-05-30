package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const cookieName = "unyport_auth"
const jwtIssuer = "unyport"

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"` // rôle indicatif; l'autorisation relit le store utilisateur.
	jwt.RegisteredClaims
}

type JWTService struct {
	key          []byte
	ttl          time.Duration
	secureCookie bool
}

func NewJWTService(b64secret string) (*JWTService, error) {
	return NewJWTServiceWithOpts(b64secret, time.Hour, false)
}

// NewJWTServiceWithOpts crée le service JWT avec TTL et flag Secure configurables.
func NewJWTServiceWithOpts(b64secret string, ttl time.Duration, secureCookie bool) (*JWTService, error) {
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
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &JWTService{key: key, ttl: ttl, secureCookie: secureCookie}, nil
}

// Issue émet un token JWT pour l'utilisateur avec son rôle embarqué.
func (j *JWTService) Issue(userID, role string) (string, time.Time, error) {
	exp := time.Now().Add(j.ttl)
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Issuer:    jwtIssuer,
			Subject:   userID,
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
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(jwtIssuer))
	if err != nil || !tok.Valid {
		return nil, errors.New("token invalide ou expiré")
	}
	if claims.UserID == "" {
		return nil, errors.New("token invalide")
	}
	return claims, nil
}

// SetCookie émet un cookie JWT pour l'utilisateur avec son rôle.
func (j *JWTService) SetCookie(w http.ResponseWriter, userID, role string) error {
	tok, exp, err := j.Issue(userID, role)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Expires:  exp,
		HttpOnly: true,
		Secure:   j.secureCookie,
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
		Expires:  time.Unix(0, 0), // epoch — force suppression même si MaxAge ignoré
		HttpOnly: true,
		Secure:   j.secureCookie,
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
