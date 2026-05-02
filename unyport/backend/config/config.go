package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type App struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Type string `yaml:"type"`
}

func (a App) TargetURL() string {
	return fmt.Sprintf("http://%s:%d", a.Host, a.Port)
}

type Config struct {
	Apps []App                       `yaml:"apps"`
	Auth map[string]map[string]string `yaml:"auth"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}