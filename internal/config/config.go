// Package config provides configuration loading and validation for jaimito.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct for jaimito.
type Config struct {
	Telegram    TelegramConfig  `yaml:"telegram"`
	Database    DatabaseConfig  `yaml:"database"`
	Channels    []ChannelConfig `yaml:"channels"`
	Server      ServerConfig    `yaml:"server"`
	SeedAPIKeys []SeedAPIKey    `yaml:"seed_api_keys"`
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

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen string `yaml:"listen"`
}

// SeedAPIKey represents a pre-seeded API key to be inserted on startup.
type SeedAPIKey struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
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

	// Default server listen address — localhost-only for VPS security.
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:8080"
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

	for _, k := range c.SeedAPIKeys {
		if k.Name == "" {
			return fmt.Errorf("seed_api_keys: name must not be empty")
		}
		if !strings.HasPrefix(k.Key, "sk-") {
			return fmt.Errorf("seed_api_keys: key for %q must start with \"sk-\"", k.Name)
		}
	}

	return nil
}

// ChannelExists reports whether a channel with the given name is configured.
func (c *Config) ChannelExists(name string) bool {
	for _, ch := range c.Channels {
		if ch.Name == name {
			return true
		}
	}
	return false
}

// ChatIDForChannel returns the chat_id for the named channel, or 0 if not found.
func (c *Config) ChatIDForChannel(name string) int64 {
	for _, ch := range c.Channels {
		if ch.Name == name {
			return ch.ChatID
		}
	}
	return 0
}
