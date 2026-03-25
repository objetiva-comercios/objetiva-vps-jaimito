package setup_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// TestDatabaseStep_Init_NewMode verifica que View contiene el titulo "Base de Datos" y el placeholder.
func TestDatabaseStep_Init_NewMode(t *testing.T) {
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "Base de Datos") {
		t.Errorf("View() debe contener 'Base de Datos'; got: %q", view)
	}
	if !strings.Contains(view, "/var/lib/jaimito/jaimito.db") {
		t.Errorf("View() debe contener placeholder '/var/lib/jaimito/jaimito.db'; got: %q", view)
	}
}

// TestDatabaseStep_Init_EditMode verifica que en modo edit se pre-llena con el valor existente.
func TestDatabaseStep_Init_EditMode(t *testing.T) {
	existingCfg := &config.Config{
		Database: config.DatabaseConfig{Path: "/custom/path/db.sqlite"},
	}
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "/custom/path/db.sqlite") {
		t.Errorf("View() en modo edit debe contener '/custom/path/db.sqlite'; got: %q", view)
	}
}

// TestDatabaseStep_EnterDefault verifica que Enter con input vacio usa el valor por defecto.
func TestDatabaseStep_EnterDefault(t *testing.T) {
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.DatabaseStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de Enter con input vacio")
	}
	if data.DatabasePath != "/var/lib/jaimito/jaimito.db" {
		t.Errorf("data.DatabasePath debe ser '/var/lib/jaimito/jaimito.db'; got: %q", data.DatabasePath)
	}
}

// TestDatabaseStep_CustomPath verifica que una ruta personalizada avanza el step.
func TestDatabaseStep_CustomPath(t *testing.T) {
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("/tmp/test.db")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.DatabaseStep)

	if !s.Done() {
		t.Error("Done() debe ser true con ruta valida")
	}
	if data.DatabasePath != "/tmp/test.db" {
		t.Errorf("data.DatabasePath debe ser '/tmp/test.db'; got: %q", data.DatabasePath)
	}
}

// TestDatabaseStep_DirMissing verifica que una ruta con dir inexistente muestra warning pero avanza.
func TestDatabaseStep_DirMissing(t *testing.T) {
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("/nonexistent/dir/test.db")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.DatabaseStep)

	// Warning no bloquea — debe avanzar
	if !s.Done() {
		t.Error("Done() debe ser true incluso con dir inexistente (es advertencia, no error)")
	}
	view := s.View(data)
	// Verificar warning mostrado
	if !strings.Contains(view, "no existe") && !strings.Contains(view, "nonexistent") {
		t.Errorf("View() debe mostrar advertencia sobre dir inexistente; got: %q", view)
	}
}

// TestDatabaseStep_EmptyPath verifica que limpiar el input y presionar Enter usa el default.
func TestDatabaseStep_EmptyPath(t *testing.T) {
	step := &setup.DatabaseStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	step.SetInputValueForTest("")

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.DatabaseStep)

	if !s.Done() {
		t.Error("Done() debe ser true con input vacio (usa default)")
	}
	if data.DatabasePath != "/var/lib/jaimito/jaimito.db" {
		t.Errorf("data.DatabasePath debe ser '/var/lib/jaimito/jaimito.db'; got: %q", data.DatabasePath)
	}
}
