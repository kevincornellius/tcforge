package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Contest struct {
	Name     string `yaml:"name"`
	Duration string `yaml:"duration"`
}

type Problem struct {
	Path        string `yaml:"path"`
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	TimeLimit   int    `yaml:"time_limit,omitempty"`   // seconds, default 1
	MemoryLimit int    `yaml:"memory_limit,omitempty"` // MB, default 256
}

type Account struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	DisplayName string `yaml:"display_name"`
}

type Judge struct {
	Languages []string `yaml:"languages"`
}

type Config struct {
	Contest  Contest   `yaml:"contest"`
	Problems []Problem `yaml:"problems"`
	Accounts []Account `yaml:"accounts"`
	Judge    Judge     `yaml:"judge"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid tcforge.yaml: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
