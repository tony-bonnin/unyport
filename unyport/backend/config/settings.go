package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Settings — TRINITY · Xen / Alpine Linux / Data Disk Mode
// Paradigme minimaliste : sécurité, prévisibilité, efficacité énergétique
type Settings struct {
	Theme string `yaml:"theme"`

	Security struct {
		CSRFSecret string `yaml:"csrf_secret"`
		JWTSecret  string `yaml:"jwt_secret"`
	} `yaml:"security"`

	Paths struct {
		UsersFile    string `yaml:"users_file"`
		LogDir       string `yaml:"log_dir"`
		LBUOverlay   string `yaml:"lbu_overlay"`
		LBUCommitLog string `yaml:"lbu_commit_log"`
	} `yaml:"paths"`

	Display struct {
		CPU       bool `yaml:"cpu"`
		Memory    bool `yaml:"memory"`
		Disk      bool `yaml:"disk"`
		Network   bool `yaml:"network"`
		XenStatus bool `yaml:"xen_status"`
		LBUStatus bool `yaml:"lbu_status"`
	} `yaml:"display"`

	LBU struct {
		Enabled    bool   `yaml:"enabled"`
		CheckDir   string `yaml:"check_dir"`
		StatusCmd  string `yaml:"status_cmd"`
		AutoDetect bool   `yaml:"auto_detect"`
	} `yaml:"lbu"`

	OAuth struct {
		Enabled        bool     `yaml:"enabled"`
		AllowedDomains []string `yaml:"allowed_domains"`
		AutoCreate     bool     `yaml:"auto_create_users"`
	} `yaml:"oauth"`

	// Security2 : hardening runtime — piloté par settings.yaml, pas par le code
	Security2 struct {
		RateLimitLogin     int      `yaml:"rate_limit_login"`
		RateLimitAPI       int      `yaml:"rate_limit_api"`
		SessionTimeoutMins int      `yaml:"session_timeout_mins"`
		HTTPS              bool     `yaml:"https"`
		TrustedOrigins     []string `yaml:"trusted_origins"`
	} `yaml:"security_extra"`

	Mail struct {
		Enabled      bool   `yaml:"enabled"`
		Host         string `yaml:"host"`
		Port         int    `yaml:"port"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		From         string `yaml:"from"`
		SendmailPath string `yaml:"sendmail_path"`
	} `yaml:"mail"`

	// HTTP3 : support QUIC/HTTP3 — requiert TLS (cert + key obligatoires)
	HTTP3 struct {
		// Enabled : active le listener QUIC sur le même port que HTTPS.
		// HTTP/1.1 + HTTP/2 restent actifs en parallèle (TCP).
		// Requiert https: true dans security_extra.
		Enabled bool `yaml:"enabled"`

		// CertFile : chemin vers le certificat TLS (PEM).
		// Ex: /etc/ssl/trinity/cert.pem
		CertFile string `yaml:"cert_file"`

		// KeyFile : chemin vers la clé privée TLS (PEM).
		// Ex: /etc/ssl/trinity/key.pem
		KeyFile string `yaml:"key_file"`

		// Port : port d'écoute HTTPS/QUIC. Défaut : 8443.
		// Le port HTTP (8800) reste ouvert pour redirection si redirect_http: true.
		Port int `yaml:"port"`

		// RedirectHTTP : redirige automatiquement HTTP → HTTPS (301).
		// Nécessite que Port soit différent de 8800.
		RedirectHTTP bool `yaml:"redirect_http"`
	} `yaml:"http3"`
}

func LoadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	// Valeurs par défaut
	if s.Paths.UsersFile == "" {
		s.Paths.UsersFile = "settings/users.json"
	}
	if s.Paths.LogDir == "" {
		s.Paths.LogDir = "logs"
	}
	if s.Theme == "" {
		s.Theme = "dark"
	}

	// LBU defaults
	if s.LBU.CheckDir == "" {
		s.LBU.CheckDir = "/media/usbdisk"
	}
	if s.LBU.StatusCmd == "" {
		s.LBU.StatusCmd = "lbu status"
	}
	if !s.Display.LBUStatus {
		s.Display.LBUStatus = true
	}

	// Security defaults
	if s.Security2.RateLimitLogin <= 0 {
		s.Security2.RateLimitLogin = 5
	}
	if s.Security2.SessionTimeoutMins <= 0 {
		s.Security2.SessionTimeoutMins = 60
	}

	// HTTP3 defaults
	if s.HTTP3.Port <= 0 {
		s.HTTP3.Port = 8443
	}

	return &s, nil
}
