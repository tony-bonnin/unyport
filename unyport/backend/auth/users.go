package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ValidRoles définit les rôles acceptés par TRINITY.
// admin    : accès total, CRUD users, toutes les pages
// operator : accès dashboard + infrastructure, lecture seule admin
// viewer   : dashboard uniquement, lecture seule
var ValidRoles = map[string]bool{
	"admin":    true,
	"operator": true,
	"viewer":   true,
}

const DefaultAdminEmail = "demo@unyport.app"

const defaultAdminPassword = "aUniC0rnForUnyPort!"

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	Roles       []string  `json:"roles"`
	DisplayName string    `json:"display_name,omitempty"`
	Avatar      string    `json:"avatar,omitempty"`    // initiales personnalisées, max 2 chars
	PhotoURL    string    `json:"photo_url,omitempty"` // URL https:// ou data URI base64 (image profil)
	SSHKey      string    `json:"ssh_key,omitempty"`   // clé publique SSH
	TwoFA       bool      `json:"two_fa"`
	CreatedAt   time.Time `json:"created_at"`
}

func (u *User) UsesDefaultCredentials() bool {
	if normalizeEmail(u.Email) != DefaultAdminEmail {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(defaultAdminPassword)) == nil
}

// Role retourne le rôle principal (premier de la liste, "viewer" par défaut).
func (u *User) Role() string {
	if len(u.Roles) == 0 {
		return "viewer"
	}
	role := strings.ToLower(strings.TrimSpace(u.Roles[0]))
	if !ValidRoles[role] {
		return "viewer"
	}
	return role
}

func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin retourne true si l'utilisateur est admin.
func (u *User) IsAdmin() bool { return u.HasRole("admin") }

// IsAtLeastOperator retourne true pour admin ou operator.
func (u *User) IsAtLeastOperator() bool {
	return u.HasRole("admin") || u.HasRole("operator")
}

// EffectiveDisplayName retourne le nom d'affichage ou la partie locale de l'email.
func (u *User) EffectiveDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	for i, c := range u.Email {
		if c == '@' {
			return u.Email[:i]
		}
	}
	return u.Email
}

// EffectiveAvatar retourne les initiales à afficher (avatar custom ou 1re lettre du nom).
func (u *User) EffectiveAvatar() string {
	if u.Avatar != "" {
		return u.Avatar
	}
	name := strings.TrimSpace(u.EffectiveDisplayName())
	if name == "" {
		return "?"
	}
	r := []rune(name)
	if len(r) == 0 {
		return "?"
	}
	return strings.ToUpper(string(r[0]))
}

type UserStore struct {
	mu    sync.Mutex
	path  string
	users map[string]*User
}

func NewUserStore(path string) (*UserStore, error) {
	s := &UserStore{path: path, users: make(map[string]*User)}
	return s, s.load()
}

func (s *UserStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return s.seedAdmin()
	}
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&s.users); err != nil {
		return err
	}
	if len(s.users) == 0 {
		return s.seedAdmin()
	}
	for email, user := range s.users {
		normalized := normalizeEmail(user.Email)
		if normalized == "" {
			normalized = normalizeEmail(email)
		}
		user.Email = normalized
		if user.ID == "" {
			user.ID = normalized
		}
		if len(user.Roles) == 0 || !ValidRoles[user.Role()] {
			user.Roles = []string{"viewer"}
		}
		if normalized != email {
			delete(s.users, email)
			s.users[normalized] = user
		}
	}
	return nil
}

// seedAdmin crée le compte admin par défaut — appelé sous mutex.
func (s *UserStore) seedAdmin() error {
	password := os.Getenv("UNYPORT_ADMIN_PASSWORD")
	if password == "" {
		password = defaultAdminPassword
		_, _ = fmt.Fprintf(os.Stderr, "UNYPORT bootstrap admin uses default credentials; change password after first login\n")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	admin := &User{
		ID:          DefaultAdminEmail,
		Email:       DefaultAdminEmail,
		Password:    string(hash),
		Roles:       []string{"admin"},
		DisplayName: "UnyPort Admin",
		CreatedAt:   time.Now(),
	}
	s.users[admin.Email] = admin
	return s.saveLocked()
}

func (s *UserStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".users-*.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.users); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	f.Close()
	return os.Rename(f.Name(), s.path)
}

func (s *UserStore) Find(email string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = normalizeEmail(email)
	u, ok := s.users[email]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (s *UserStore) List() []*User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

func (s *UserStore) Add(u *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.Email = normalizeEmail(u.Email)
	if u.Email == "" {
		return errors.New("email required")
	}
	if u.ID == "" {
		u.ID = u.Email
	}
	if _, exists := s.users[u.Email]; exists {
		return errors.New("user already exists")
	}
	if u.Password != "" {
		if len(u.Password) < 8 {
			return errors.New("mot de passe trop court (8 caractères minimum)")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			return err
		}
		u.Password = string(hash)
	}
	if !ValidRoles[u.Role()] {
		u.Roles = []string{"viewer"}
	}
	s.users[u.Email] = u
	return s.saveLocked()
}

func (s *UserStore) Delete(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = normalizeEmail(email)
	if _, ok := s.users[email]; !ok {
		return errors.New("user not found")
	}
	delete(s.users, email)
	return s.saveLocked()
}

// UpdateProfile met à jour les champs modifiables par l'utilisateur lui-même.
// Retourne l'utilisateur mis à jour et indique si l'email a changé.
func (s *UserStore) UpdateProfile(email, newEmail, displayName, avatar, photoURL, sshKey string) (*User, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = normalizeEmail(email)
	newEmail = normalizeEmail(newEmail)
	if newEmail == "" {
		newEmail = email
	}
	if !looksLikeEmail(newEmail) {
		return nil, false, errors.New("email invalid")
	}
	u, ok := s.users[email]
	if !ok {
		return nil, false, errors.New("user not found")
	}
	emailChanged := newEmail != email
	if emailChanged {
		if _, exists := s.users[newEmail]; exists {
			return nil, false, errors.New("email already exists")
		}
	}
	u.DisplayName = displayName
	if len([]rune(avatar)) > 2 {
		return nil, false, errors.New("avatar: 2 caractères maximum")
	}
	if photoURL != "" {
		if len(photoURL) > 2*1024*1024 {
			return nil, false, errors.New("photo: taille maximale 2 Mo")
		}
		if !strings.HasPrefix(photoURL, "https://") &&
			!strings.HasPrefix(photoURL, "data:image/") {
			return nil, false, errors.New("photo: URL https ou data URI image requis")
		}
	}
	if emailChanged {
		delete(s.users, email)
		u.Email = newEmail
		u.ID = newEmail
		s.users[newEmail] = u
	}
	u.Avatar = avatar
	u.PhotoURL = photoURL
	u.SSHKey = sshKey
	return u, emailChanged, s.saveLocked()
}

// UpdatePassword vérifie l'ancien mot de passe et remplace par le nouveau.
func (s *UserStore) UpdatePassword(email, oldPwd, newPwd string) error {
	if len(newPwd) < 8 {
		return errors.New("mot de passe trop court (8 caractères minimum)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	email = normalizeEmail(email)
	u, ok := s.users[email]
	if !ok {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPwd)); err != nil {
		return errors.New("mot de passe actuel incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), 12)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return s.saveLocked()
}

// SetRole assigne un rôle unique à un utilisateur (admin seulement).
func (s *UserStore) SetRole(email, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if !ValidRoles[role] {
		return errors.New("rôle invalide (admin|operator|viewer)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	email = normalizeEmail(email)
	u, ok := s.users[email]
	if !ok {
		return errors.New("user not found")
	}
	u.Roles = []string{role}
	return s.saveLocked()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func looksLikeEmail(email string) bool {
	if strings.Count(email, "@") != 1 {
		return false
	}
	parts := strings.Split(email, "@")
	return parts[0] != "" && strings.Contains(parts[1], ".")
}

func randomPassword() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b[:]), nil
}
