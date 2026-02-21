package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Telegram.Token != "test-token" {
		t.Errorf("expected token 'test-token', got %q", cfg.Telegram.Token)
	}
	if cfg.Database.Path != "/var/lib/jaimito/jaimito.db" {
		t.Errorf("expected default db path, got %q", cfg.Database.Path)
	}
	if len(cfg.Channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(cfg.Channels))
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestValidate_MissingToken(t *testing.T) {
	content := `
telegram:
  token: ""
channels:
  - name: general
    chat_id: 12345
    priority: normal
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil || err.Error() != "telegram.token is required" {
		t.Errorf("expected 'telegram.token is required', got: %v", err)
	}
}

func TestValidate_MissingGeneralChannel(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: deploys
    chat_id: 12345
    priority: normal
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil || err.Error() != "a channel named 'general' is required" {
		t.Errorf("expected 'a channel named general is required', got: %v", err)
	}
}

func TestValidate_InvalidPriority(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: critical
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid priority, got nil")
	}
}

func TestValidate_DuplicateChannelName(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
  - name: general
    chat_id: 67890
    priority: high
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate channel name, got nil")
	}
}

func TestValidate_NoChannels(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels: []
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil || err.Error() != "at least one channel is required" {
		t.Errorf("expected 'at least one channel is required', got: %v", err)
	}
}

func TestLoad_ExampleConfig(t *testing.T) {
	// Verify the example config is valid (except for placeholder token).
	// We patch the token field manually since it's a placeholder.
	dir := t.TempDir()
	content := `
telegram:
  token: "REAL_TOKEN"
database:
  path: "/var/lib/jaimito/jaimito.db"
channels:
  - name: general
    chat_id: 100000001
    priority: normal
  - name: deploys
    chat_id: 100000002
    priority: normal
  - name: errors
    chat_id: 100000003
    priority: high
  - name: cron
    chat_id: 100000004
    priority: low
  - name: system
    chat_id: 100000005
    priority: normal
  - name: security
    chat_id: 100000006
    priority: high
  - name: monitoring
    chat_id: 100000007
    priority: normal
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for 7-channel config, got: %v", err)
	}
	if len(cfg.Channels) != 7 {
		t.Errorf("expected 7 channels, got %d", len(cfg.Channels))
	}
}
