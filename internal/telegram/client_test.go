package telegram_test

import (
	"context"
	"testing"

	"github.com/chiguire/jaimito/internal/telegram"
)

// TestValidateTokenWithInfo_Format verifica que un token vacio retorna error de formato.
func TestValidateTokenWithInfo_Format(t *testing.T) {
	ctx := context.Background()
	_, _, err := telegram.ValidateTokenWithInfo(ctx, "")
	if err == nil {
		t.Fatal("esperaba error para token vacio, got nil")
	}
	if !contains(err.Error(), "formato de token invalido") {
		t.Errorf("error esperado contener 'formato de token invalido', got: %s", err.Error())
	}
}

// TestBotInfo_Fields verifica que BotInfo tiene los campos Username y DisplayName.
func TestBotInfo_Fields(t *testing.T) {
	info := telegram.BotInfo{
		Username:    "testbot",
		DisplayName: "Test Bot",
	}
	if info.Username != "testbot" {
		t.Errorf("Username = %q, esperaba %q", info.Username, "testbot")
	}
	if info.DisplayName != "Test Bot" {
		t.Errorf("DisplayName = %q, esperaba %q", info.DisplayName, "Test Bot")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
