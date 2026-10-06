package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
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
