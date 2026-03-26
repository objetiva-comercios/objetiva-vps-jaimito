// Package config provides configuration loading and validation for jaimito.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct for jaimito.
type Config struct {
	Telegram    TelegramConfig  `yaml:"telegram"`
	Database    DatabaseConfig  `yaml:"database"`
	Channels    []ChannelConfig `yaml:"channels"`
	Server      ServerConfig    `yaml:"server"`
	SeedAPIKeys []SeedAPIKey    `yaml:"seed_api_keys"`
	Metrics     *MetricsConfig  `yaml:"metrics,omitempty"`
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

// MetricsConfig holds the optional metrics collection configuration.
// If this section is absent from config.yaml, metrics are disabled entirely (D-04).
type MetricsConfig struct {
	Retention       string      `yaml:"retention"`
	AlertCooldown   string      `yaml:"alert_cooldown"`
	CollectInterval string      `yaml:"collect_interval"`
	Definitions     []MetricDef `yaml:"definitions"`
}

// Thresholds defines warning and critical thresholds for a metric.
// Both fields use pointers so that omitting them in YAML results in nil (no alerts).
type Thresholds struct {
	Warning  *float64 `yaml:"warning"`
	Critical *float64 `yaml:"critical"`
}

// MetricDef describes a single metric to collect via a shell command.
type MetricDef struct {
	Name       string      `yaml:"name"`
	Command    string      `yaml:"command"`
	Interval   string      `yaml:"interval,omitempty"`   // inherits collect_interval if empty
	Category   string      `yaml:"category,omitempty"`   // default "custom" applied at runtime
	Type       string      `yaml:"type,omitempty"`       // default "gauge" applied at runtime
	Thresholds *Thresholds `yaml:"thresholds,omitempty"` // nil = no alerts for this metric
}

// parseDuration converts duration strings like "7d", "300s", "5m" to time.Duration.
// Go's time.ParseDuration does not support "d" (days), so this function handles it specially.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("duration must not be empty")
	}
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q: days must be a positive integer", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid duration %q: must be positive", s)
	}
	return d, nil
}

// ParseDuration is the exported wrapper around parseDuration for use by Phase 9+ packages.
func ParseDuration(s string) (time.Duration, error) {
	return parseDuration(s)
}

// validate checks the MetricsConfig fields for correctness.
func (m *MetricsConfig) validate() error {
	if _, err := parseDuration(m.Retention); err != nil {
		return fmt.Errorf("metrics.retention: %w", err)
	}
	if _, err := parseDuration(m.AlertCooldown); err != nil {
		return fmt.Errorf("metrics.alert_cooldown: %w", err)
	}
	collectInterval, err := parseDuration(m.CollectInterval)
	if err != nil {
		return fmt.Errorf("metrics.collect_interval: %w", err)
	}

	seen := make(map[string]bool)
	for i, def := range m.Definitions {
		if def.Name == "" {
			return fmt.Errorf("metrics.definitions[%d]: name must not be empty", i)
		}
		if seen[def.Name] {
			return fmt.Errorf("metrics.definitions[%d]: duplicate name %q", i, def.Name)
		}
		seen[def.Name] = true
		if def.Command == "" {
			return fmt.Errorf("metrics.definitions[%d] %q: command must not be empty", i, def.Name)
		}
		if def.Interval != "" {
			d, err := parseDuration(def.Interval)
			if err != nil {
				return fmt.Errorf("metrics.definitions[%d] %q: interval: %w", i, def.Name, err)
			}
			if d <= 0 {
				return fmt.Errorf("metrics.definitions[%d] %q: interval must be positive", i, def.Name)
			}
		} else {
			// Definition inherits the global collect_interval; already validated above.
			_ = collectInterval
		}
		if def.Thresholds != nil && def.Thresholds.Warning != nil && def.Thresholds.Critical != nil {
			if *def.Thresholds.Warning >= *def.Thresholds.Critical {
				return fmt.Errorf("metrics.definitions[%d] %q: thresholds.warning must be < critical", i, def.Name)
			}
		}
	}
	return nil
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

	if c.Metrics != nil {
		if err := c.Metrics.validate(); err != nil {
			return err
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
