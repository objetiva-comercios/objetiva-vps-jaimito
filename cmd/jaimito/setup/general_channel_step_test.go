package setup_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// TestGeneralChannelStep_Init verifica que Init configura el step con placeholder correcto.
func TestGeneralChannelStep_Init(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "Canal General") {
		t.Errorf("View() debe contener 'Canal General'; got: %q", view)
	}
}

// TestGeneralChannelStep_ValidChat verifica que un resultado exitoso setea done=true
// y agrega ChannelConfig{Name:"general", Priority:"normal"} a data.Channels.
func TestGeneralChannelStep_ValidChat(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Simular estado de validacion en progreso con seq=1
	step.SetValidationState(1, true)

	msg := setup.NewChatValidationResultMsg(1, -1001234567890, "Test Group", "supergroup", nil)

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.GeneralChannelStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de resultado valido")
	}

	if len(data.Channels) == 0 {
		t.Fatal("data.Channels debe tener al menos 1 canal")
	}
	ch := data.Channels[0]
	if ch.Name != "general" {
		t.Errorf("canal.Name = %q, esperaba %q", ch.Name, "general")
	}
	if ch.ChatID != -1001234567890 {
		t.Errorf("canal.ChatID = %d, esperaba %d", ch.ChatID, int64(-1001234567890))
	}
	if ch.Priority != "normal" {
		t.Errorf("canal.Priority = %q, esperaba %q", ch.Priority, "normal")
	}
}

// TestGeneralChannelStep_InvalidChat verifica que un error de validacion muestra error y done=false.
func TestGeneralChannelStep_InvalidChat(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	step.SetValidationState(1, true)

	msg := setup.NewChatValidationResultMsg(1, -1001234567890, "", "", errors.New("bot no tiene acceso al chat"))

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.GeneralChannelStep)

	if s.Done() {
		t.Error("Done() debe ser false despues de resultado con error")
	}
	view := s.View(data)
	if !strings.Contains(view, "no tiene acceso") {
		t.Errorf("View() debe contener el error; got: %q", view)
	}
}

// TestGeneralChannelStep_StaleResponse verifica que una respuesta stale (seq no coincide) no cambia el estado.
func TestGeneralChannelStep_StaleResponse(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// seq actual es 2, respuesta stale tiene seq=1
	step.SetValidationState(2, true)

	msg := setup.NewChatValidationResultMsg(1, -1001234567890, "Test Group", "supergroup", nil)

	updated, _ := step.Update(msg, data)
	s := updated.(*setup.GeneralChannelStep)

	if s.Done() {
		t.Error("Done() debe ser false para respuesta stale")
	}
	if len(data.Channels) != 0 {
		t.Error("data.Channels no debe modificarse con respuesta stale")
	}
}

// TestGeneralChannelStep_InvalidFormat verifica que texto no-numerico muestra error sin lanzar validacion.
func TestGeneralChannelStep_InvalidFormat(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Setear valor no-numerico en el input a traves de teclas
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	// Primero simular que se tipeo texto invalido
	step.SetInputValueForTest("no-es-numerico")

	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.GeneralChannelStep)

	if s.Done() {
		t.Error("Done() debe ser false con formato invalido")
	}
	view := s.View(data)
	if !strings.Contains(view, "numerico") {
		t.Errorf("View() debe contener error de formato numerico; got: %q", view)
	}
}

// TestGeneralChannelStep_EditModeNoChange verifica que en modo edit, Enter sin cambiar mantiene el existente.
func TestGeneralChannelStep_EditModeNoChange(t *testing.T) {
	existingCfg := &config.Config{
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: -1001234567890, Priority: "normal"},
		},
	}
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	step.Init(data)

	// Enviar Enter sin cambiar el chat ID
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.GeneralChannelStep)

	if !s.Done() {
		t.Error("Done() debe ser true en modo edit al presionar Enter sin cambiar el chat ID")
	}
	if len(data.Channels) == 0 {
		t.Fatal("data.Channels debe tener al menos 1 canal")
	}
	if data.Channels[0].ChatID != -1001234567890 {
		t.Errorf("data.Channels[0].ChatID = %d, esperaba %d", data.Channels[0].ChatID, int64(-1001234567890))
	}
}

// TestGeneralChannelStep_ViewValidated verifica que la vista cuando done=true muestra titulo y tipo del chat.
func TestGeneralChannelStep_ViewValidated(t *testing.T) {
	step := &setup.GeneralChannelStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)
	step.SetDoneForTest(true)
	step.SetChatInfoForTest("Test Group", "supergroup")

	view := step.View(data)
	if !strings.Contains(view, "Test Group") {
		t.Errorf("View() debe contener 'Test Group'; got: %q", view)
	}
	if !strings.Contains(view, "supergroup") {
		t.Errorf("View() debe contener 'supergroup'; got: %q", view)
	}
}
