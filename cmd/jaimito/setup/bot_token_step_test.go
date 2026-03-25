package setup_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/telegram"
)

// TestBotTokenStep_Init verifica que Init configura el step correctamente.
func TestBotTokenStep_Init(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "Bot Token") {
		t.Errorf("View() debe contener 'Bot Token'; got: %q", view)
	}
	// El step muestra el campo de input y el hint de BotFather
	if !strings.Contains(view, "BotFather") {
		t.Errorf("View() debe contener hint de BotFather; got: %q", view)
	}
}

// TestBotTokenStep_ValidResult verifica que un resultado exitoso setea done=true y campos en data.
func TestBotTokenStep_ValidResult(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Simular estado de validacion en progreso con seq=1
	step.SetValidationState(1, true)

	msg := setup.NewTokenValidationResultMsg(1, telegram.BotInfo{Username: "testbot", DisplayName: "Test Bot"}, nil)

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.BotTokenStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de resultado valido")
	}
	if data.BotUsername != "testbot" {
		t.Errorf("data.BotUsername = %q, esperaba %q", data.BotUsername, "testbot")
	}
	if data.BotDisplayName != "Test Bot" {
		t.Errorf("data.BotDisplayName = %q, esperaba %q", data.BotDisplayName, "Test Bot")
	}
}

// TestBotTokenStep_InvalidResult verifica que un resultado con error setea validError y done=false.
func TestBotTokenStep_InvalidResult(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	step.SetValidationState(1, true)

	msg := setup.NewTokenValidationResultMsg(1, telegram.BotInfo{}, errors.New("token invalido"))

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.BotTokenStep)

	if s.Done() {
		t.Error("Done() debe ser false despues de resultado con error")
	}
	view := s.View(data)
	if !strings.Contains(view, "token invalido") {
		t.Errorf("View() debe contener el error; got: %q", view)
	}
}

// TestBotTokenStep_StaleResponse verifica que una respuesta stale (seq no coincide) no cambia el estado.
func TestBotTokenStep_StaleResponse(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// seq actual es 2, respuesta stale tiene seq=1
	step.SetValidationState(2, true)

	msg := setup.NewTokenValidationResultMsg(1, telegram.BotInfo{Username: "testbot", DisplayName: "Test Bot"}, nil)

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.BotTokenStep)

	if s.Done() {
		t.Error("Done() debe ser false para respuesta stale")
	}
	if data.BotUsername != "" {
		t.Errorf("data.BotUsername no debe modificarse con respuesta stale; got: %q", data.BotUsername)
	}
}

// TestBotTokenStep_EditModeNoChange verifica que en modo edit, Enter sin cambiar token avanza sin re-validar.
func TestBotTokenStep_EditModeNoChange(t *testing.T) {
	existingCfg := &config.Config{
		Telegram: config.TelegramConfig{Token: "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"},
	}
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	step.Init(data)

	// Enviar Enter sin cambiar el token (el valor del input tiene el token ofuscado, no lo cambio)
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.BotTokenStep)

	if !s.Done() {
		t.Error("Done() debe ser true en modo edit al presionar Enter sin cambiar el token")
	}
	if data.BotToken != "123456789:ABCdefGhIjKlMnOpQrStUvWxYz" {
		t.Errorf("data.BotToken debe ser el token original; got: %q", data.BotToken)
	}
}

// TestBotTokenStep_ViewValidated verifica que la vista cuando done=true muestra @username y display name.
func TestBotTokenStep_ViewValidated(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{
		Mode:           "new",
		BotUsername:    "testbot",
		BotDisplayName: "Test Bot",
	}
	step.Init(data)
	step.SetDoneForTest(true)

	view := step.View(data)
	if !strings.Contains(view, "@testbot") {
		t.Errorf("View() debe contener '@testbot'; got: %q", view)
	}
	if !strings.Contains(view, "Test Bot") {
		t.Errorf("View() debe contener 'Test Bot'; got: %q", view)
	}
}

// TestBotTokenStep_ViewValidating verifica que la vista durante validacion muestra el texto de spinner.
func TestBotTokenStep_ViewValidating(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)
	step.SetValidationState(1, true)

	view := step.View(data)
	if !strings.Contains(view, "Validando bot token") {
		t.Errorf("View() debe contener 'Validando bot token'; got: %q", view)
	}
}

// TestBotTokenStep_ViewError verifica que la vista con error muestra el error.
func TestBotTokenStep_ViewError(t *testing.T) {
	step := &setup.BotTokenStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)
	step.SetValidErrorForTest("formato de token invalido: error")

	view := step.View(data)
	if !strings.Contains(view, "formato de token invalido") {
		t.Errorf("View() debe contener el error; got: %q", view)
	}
}
