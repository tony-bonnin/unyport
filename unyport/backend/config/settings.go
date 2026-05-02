package config

import (
	"os"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	OS    string `yaml:"os"`
	Theme string `yaml:"theme"`

	Security struct {
		CSRFSecret string `yaml:"csrf_secret"`
		JWTSecret  string `yaml:"jwt_secret"`
	} `yaml:"security"`

	Paths struct {
		UsersFile string `yaml:"users_file"`
		LogDir    string `yaml:"log_dir"`
	} `yaml:"paths"`

	Display struct {
		CPU     bool `yaml:"cpu"`
		Memory  bool `yaml:"memory"`
		Disk    bool `yaml:"disk"`
		Network bool `yaml:"network"`
	} `yaml:"display"`
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
	if s.OS == "auto" || s.OS == "" {
		s.OS = detectOS()
	}
	if s.Paths.UsersFile == "" {
		s.Paths.UsersFile = "settings/users.json"
	}
	if s.Paths.LogDir == "" {
		s.Paths.LogDir = "logs"
	}
	return &s, nil
}

func detectOS() string {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return "linux"
		}
		lower := strings.ToLower(string(data))
		for _, id := range []string{"alpine", "debian", "ubuntu", "centos", "fedora", "arch", "manjaro", "opensuse"} {
			if strings.Contains(lower, id) {
				return id
			}
		}
		if strings.Contains(lower, "red hat") || strings.Contains(lower, "rhel") {
			return "rhel"
		}
		return "linux"
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "freebsd":
		return "freebsd"
	default:
		return runtime.GOOS
	}
}