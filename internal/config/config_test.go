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

func TestLoad_DefaultServerListen(t *testing.T) {
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
	if cfg.Server.Listen != "127.0.0.1:8080" {
		t.Errorf("expected default server.listen '127.0.0.1:8080', got %q", cfg.Server.Listen)
	}
}

func TestLoad_ExplicitServerListen(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
server:
  listen: "0.0.0.0:9090"
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Server.Listen != "0.0.0.0:9090" {
		t.Errorf("expected server.listen '0.0.0.0:9090', got %q", cfg.Server.Listen)
	}
}

func TestChannelExists(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
  - name: deploys
    chat_id: 67890
    priority: high
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.ChannelExists("general") {
		t.Error("expected ChannelExists('general') to return true")
	}
	if !cfg.ChannelExists("deploys") {
		t.Error("expected ChannelExists('deploys') to return true")
	}
	if cfg.ChannelExists("missing") {
		t.Error("expected ChannelExists('missing') to return false")
	}
}

func TestChatIDForChannel(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
  - name: deploys
    chat_id: 67890
    priority: high
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if id := cfg.ChatIDForChannel("general"); id != 12345 {
		t.Errorf("expected ChatIDForChannel('general') = 12345, got %d", id)
	}
	if id := cfg.ChatIDForChannel("deploys"); id != 67890 {
		t.Errorf("expected ChatIDForChannel('deploys') = 67890, got %d", id)
	}
	if id := cfg.ChatIDForChannel("missing"); id != 0 {
		t.Errorf("expected ChatIDForChannel('missing') = 0, got %d", id)
	}
}

func TestValidate_SeedAPIKeys_Valid(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
seed_api_keys:
  - name: "ci-bot"
    key: "sk-abc123"
  - name: "deploy-agent"
    key: "sk-xyz789"
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for valid seed_api_keys, got: %v", err)
	}
	if len(cfg.SeedAPIKeys) != 2 {
		t.Errorf("expected 2 seed api keys, got %d", len(cfg.SeedAPIKeys))
	}
}

func TestValidate_SeedAPIKeys_MissingName(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
seed_api_keys:
  - name: ""
    key: "sk-abc123"
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing seed api key name, got nil")
	}
}

func TestValidate_SeedAPIKeys_MissingSkPrefix(t *testing.T) {
	content := `
telegram:
  token: "test-token"
channels:
  - name: general
    chat_id: 12345
    priority: normal
seed_api_keys:
  - name: "ci-bot"
    key: "invalid-key"
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for key missing sk- prefix, got nil")
	}
}
