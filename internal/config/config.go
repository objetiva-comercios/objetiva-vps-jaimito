// Package config provides configuration loading and validation for jaimito.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct for jaimito.
type Config struct {
	Telegram TelegramConfig  `yaml:"telegram"`
	Database DatabaseConfig  `yaml:"database"`
	Channels []ChannelConfig `yaml:"channels"`
}

// TelegramConfig holds Telegram bot credentials.
type TelegramConfig struct {
	Token string `yaml:"token"`
}

// DatabaseConfig holds SQLite database settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ChannelConfig represents a single notification channel.
type ChannelConfig struct {
	Name     string `yaml:"name"`
	ChatID   int64  `yaml:"chat_id"`
	Priority string `yaml:"priority"`
}

// validPriorities is the set of accepted priority values.
var validPriorities = map[string]bool{
	"low":    true,
	"normal": true,
	"high":   true,
}

// Load reads and parses a YAML config file at the given path.
// It sets default values and validates the resulting configuration.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Set default database path if not specified.
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/var/lib/jaimito/jaimito.db"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that all required fields are present and valid.
// It returns the first validation error encountered.
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}

	if len(c.Channels) == 0 {
		return fmt.Errorf("at least one channel is required")
	}

	hasGeneral := false
	seen := make(map[string]bool)

	for _, ch := range c.Channels {
		if ch.Name == "" {
			return fmt.Errorf("channel name must not be empty")
		}

		if seen[ch.Name] {
			return fmt.Errorf("duplicate channel name: %q", ch.Name)
		}
		seen[ch.Name] = true

		if ch.ChatID == 0 {
			return fmt.Errorf("channel %q: chat_id must not be zero", ch.Name)
		}

		if !validPriorities[ch.Priority] {
			return fmt.Errorf("channel %q: invalid priority %q (must be low, normal, or high)", ch.Name, ch.Priority)
		}

		if ch.Name == "general" {
			hasGeneral = true
		}
	}

	if !hasGeneral {
		return fmt.Errorf("a channel named 'general' is required")
	}

	return nil
}
