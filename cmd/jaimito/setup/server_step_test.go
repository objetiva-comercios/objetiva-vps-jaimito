package setup_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// TestServerStep_Init_NewMode verifica que View contiene el titulo "Servidor" y el placeholder.
func TestServerStep_Init_NewMode(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "Servidor") {
		t.Errorf("View() debe contener 'Servidor'; got: %q", view)
	}
	if !strings.Contains(view, "127.0.0.1:8080") {
		t.Errorf("View() debe contener placeholder '127.0.0.1:8080'; got: %q", view)
	}
}

// TestServerStep_Init_EditMode verifica que en modo edit se pre-llena con el valor existente.
func TestServerStep_Init_EditMode(t *testing.T) {
	existingCfg := &config.Config{
		Server: config.ServerConfig{Listen: "0.0.0.0:9090"},
	}
	step := &setup.ServerStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "0.0.0.0:9090") {
		t.Errorf("View() en modo edit debe contener '0.0.0.0:9090'; got: %q", view)
	}
}

// TestServerStep_EnterDefault verifica que Enter con input vacio usa el valor por defecto.
func TestServerStep_EnterDefault(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.ServerStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de Enter con input vacio")
	}
	if data.ServerListen != "127.0.0.1:8080" {
		t.Errorf("data.ServerListen debe ser '127.0.0.1:8080'; got: %q", data.ServerListen)
	}
}

// TestServerStep_ValidAddress verifica que una direccion valida avanza el step.
func TestServerStep_ValidAddress(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("0.0.0.0:9090")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.ServerStep)

	if !s.Done() {
		t.Error("Done() debe ser true con direccion valida")
	}
	if data.ServerListen != "0.0.0.0:9090" {
		t.Errorf("data.ServerListen debe ser '0.0.0.0:9090'; got: %q", data.ServerListen)
	}
}

// TestServerStep_InvalidFormat verifica que un formato invalido muestra error y Done()=false.
func TestServerStep_InvalidFormat(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("noport")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.ServerStep)

	if s.Done() {
		t.Error("Done() debe ser false con formato invalido")
	}
	view := s.View(data)
	if !strings.Contains(view, "Formato invalido") && !strings.Contains(view, "invalido") {
		t.Errorf("View() debe mostrar error de formato; got: %q", view)
	}
}

// TestServerStep_InvalidPort verifica que un puerto fuera de rango muestra error.
func TestServerStep_InvalidPort(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("127.0.0.1:99999")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.ServerStep)

	if s.Done() {
		t.Error("Done() debe ser false con puerto invalido")
	}
	view := s.View(data)
	if !strings.Contains(view, "rango") && !strings.Contains(view, "65535") {
		t.Errorf("View() debe mostrar error de rango de puerto; got: %q", view)
	}
}

// TestServerStep_EmptyAfterTyping verifica que limpiar el input y presionar Enter usa el default.
func TestServerStep_EmptyAfterTyping(t *testing.T) {
	step := &setup.ServerStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	// Setear y limpiar
	step.SetInputValueForTest("")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.ServerStep)

	if !s.Done() {
		t.Error("Done() debe ser true al presionar Enter con input vacio (usa default)")
	}
	if data.ServerListen != "127.0.0.1:8080" {
		t.Errorf("data.ServerListen debe ser '127.0.0.1:8080'; got: %q", data.ServerListen)
	}
}
