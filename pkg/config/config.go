package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	API struct {
		BaseURL string `yaml:"base_url"`
		APIKey  string `yaml:"api_key"`
		Model   string `yaml:"model"`
	} `yaml:"api"`
	OCR struct {
		Language string `yaml:"language"`
		Prompt   string `yaml:"prompt"`
	} `yaml:"ocr"`
	Workers   int    `yaml:"workers"`
	Format    string `yaml:"format"`
	Debug     bool   `yaml:"debug"`
	PNGExport string `yaml:"png_export"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Workers = 4
	cfg.Format = "ass"
	cfg.OCR.Language = "English"
	cfg.API.BaseURL = "https://api.openai.com"
	cfg.API.Model = "gpt-4o"
	cfg.Debug = false
	cfg.PNGExport = ""
	return cfg
}

// Load reads config from a YAML file. If path is empty, it searches:
// 1. ./linotype.yaml (current working directory)
// 2. ~/.config/linotype/config.yaml
// If no config file is found, returns DefaultConfig with no error.
func Load(path string) (*Config, error) {
	var pathsToTry []string

	if path != "" {
		pathsToTry = append(pathsToTry, path)
	} else {
		pathsToTry = append(pathsToTry, "linotype.yaml")
		home, err := os.UserHomeDir()
		if err == nil {
			pathsToTry = append(pathsToTry, filepath.Join(home, ".config", "linotype", "config.yaml"))
		}
	}

	for _, p := range pathsToTry {
		data, err := os.ReadFile(p)
		if err == nil {
			cfg := DefaultConfig()
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config %s: %w", p, err)
			}
			return cfg, nil
		}
	}

	return DefaultConfig(), nil
}

