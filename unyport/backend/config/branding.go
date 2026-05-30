package config

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// BrandingColors — couleurs des rôles Xen / Alpine.
// Chaque valeur est un code hex CSS (#rrggbb).
// Valeurs par défaut alignées sur style.css [data-role].
type BrandingColors struct {
	Dom0      string `yaml:"dom0"`      // défaut : #3e3aab
	DomU      string `yaml:"domu"`      // défaut : #a0284a
	Container string `yaml:"container"` // défaut : #b05a1a
	Alpine    string `yaml:"alpine"`    // défaut : #28587c
}

// Branding — identité visuelle de l'instance TRINITY.
// Stocké dans branding.yaml, persisté par LBU.
//
// Logo : priorité LogoBase64 > LogoURL > logo embarqué.
// LogoBase64 : data URI complet "data:image/...;base64,..."
// LogoURL    : URL http/https externe
type Branding struct {
	LogoBase64 string         `yaml:"logo_base64"` // data URI (png/svg/webp, max 2 Mo)
	LogoURL    string         `yaml:"logo_url"`    // URL http/https alternative
	Colors     BrandingColors `yaml:"colors"`
}

// EffectiveLogoSrc retourne la source du logo à injecter dans src="…".
// Retourne "" si aucun logo personnalisé — le frontend utilise le logo par défaut.
func (b *Branding) EffectiveLogoSrc() string {
	if b.LogoBase64 != "" {
		return b.LogoBase64
	}
	if b.LogoURL != "" {
		return b.LogoURL
	}
	return ""
}

// HasCustomLogo retourne true si un logo personnalisé est défini.
func (b *Branding) HasCustomLogo() bool {
	return b.LogoBase64 != "" || b.LogoURL != ""
}

// defaults applique les couleurs par défaut si non renseignées.
func (b *Branding) defaults() {
	if b.Colors.Dom0 == "" {
		b.Colors.Dom0 = "#3e3aab"
	}
	if b.Colors.DomU == "" {
		b.Colors.DomU = "#a0284a"
	}
	if b.Colors.Container == "" {
		b.Colors.Container = "#b05a1a"
	}
	if b.Colors.Alpine == "" {
		b.Colors.Alpine = "#28587c"
	}
}

// BrandingStore gère la lecture/écriture thread-safe de branding.yaml.
type BrandingStore struct {
	mu       sync.RWMutex
	path     string
	branding Branding
}

func NewBrandingStore(path string) (*BrandingStore, error) {
	s := &BrandingStore{path: path}
	return s, s.load()
}

func (s *BrandingStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// Pas de fichier → branding par défaut
		s.branding.defaults()
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &s.branding); err != nil {
		return err
	}
	s.branding.defaults()
	return nil
}

// Get retourne une copie du branding courant.
func (s *BrandingStore) Get() Branding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.branding
}

// Update valide et persiste un nouveau branding.
func (s *BrandingStore) Update(b Branding) error {
	if err := validateBranding(&b); err != nil {
		return err
	}
	b.defaults()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.branding = b

	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}

	out, err := yaml.Marshal(&b)
	if err != nil {
		return err
	}

	// Écriture atomique via fichier temporaire
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".branding-*.yaml")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), s.path)
}

// validateBranding vérifie les contraintes sur les champs.
func validateBranding(b *Branding) error {
	// Logo
	if b.LogoBase64 != "" {
		if len(b.LogoBase64) > 2*1024*1024 {
			return errors.New("logo: taille maximale 2 Mo")
		}
		if !strings.HasPrefix(b.LogoBase64, "data:image/") {
			return errors.New("logo: data URI image requis (data:image/...)")
		}
		// Vérifier que la partie base64 est décodable
		parts := strings.SplitN(b.LogoBase64, ",", 2)
		if len(parts) != 2 {
			return errors.New("logo: data URI malformé")
		}
		if _, err := base64.StdEncoding.DecodeString(parts[1]); err != nil {
			// Essayer RawStdEncoding (sans padding)
			if _, err2 := base64.RawStdEncoding.DecodeString(parts[1]); err2 != nil {
				return errors.New("logo: base64 invalide")
			}
		}
	}

	if b.LogoURL != "" {
		if !strings.HasPrefix(b.LogoURL, "http://") && !strings.HasPrefix(b.LogoURL, "https://") {
			return errors.New("logo_url: URL http/https requise")
		}
	}

	// Couleurs — format hex basique
	for name, color := range map[string]string{
		"dom0": b.Colors.Dom0, "domu": b.Colors.DomU,
		"container": b.Colors.Container, "alpine": b.Colors.Alpine,
	} {
		if color != "" && !isHexColor(color) {
			return errors.New("color " + name + ": format #rrggbb requis")
		}
	}

	return nil
}

// isHexColor vérifie #rgb ou #rrggbb.
func isHexColor(s string) bool {
	if len(s) == 0 || s[0] != '#' {
		return false
	}
	rest := s[1:]
	if len(rest) != 3 && len(rest) != 6 {
		return false
	}
	for _, c := range rest {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}