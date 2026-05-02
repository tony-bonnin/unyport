package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	Roles     []string  `json:"roles"`
	TwoFA     bool      `json:"two_fa"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
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
	return nil
}

// seedAdmin crée le compte admin par défaut — appelé sous mutex.
func (s *UserStore) seedAdmin() error {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		return err
	}
	admin := &User{
		ID:        "admin",
		Email:     "admin@unyport.local",
		Password:  string(hash),
		Roles:     []string{"admin"},
		CreatedAt: time.Now(),
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
	if _, exists := s.users[u.Email]; exists {
		return errors.New("user already exists")
	}
	if u.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			return err
		}
		u.Password = string(hash)
	}
	s.users[u.Email] = u
	return s.saveLocked()
}

func (s *UserStore) Delete(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("user not found")
	}
	delete(s.users, email)
	return s.saveLocked()
}