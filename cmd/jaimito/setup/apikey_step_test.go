package setup_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// TestAPIKeyStep_Init_NewMode verifica que Init genera key y View la contiene.
func TestAPIKeyStep_Init_NewMode(t *testing.T) {
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	if !strings.HasPrefix(data.GeneratedAPIKey, "sk-") {
		t.Errorf("data.GeneratedAPIKey debe empezar con 'sk-'; got: %q", data.GeneratedAPIKey)
	}
	if len(data.GeneratedAPIKey) != 67 {
		t.Errorf("data.GeneratedAPIKey debe tener 67 chars; got: %d", len(data.GeneratedAPIKey))
	}

	view := step.View(data)
	if !strings.Contains(view, data.GeneratedAPIKey) {
		t.Errorf("View() debe contener la key generada %q; got: %q", data.GeneratedAPIKey, view)
	}
}

// TestAPIKeyStep_View_ShowsKeyBox verifica que View muestra la key y el warning ATENCION.
func TestAPIKeyStep_View_ShowsKeyBox(t *testing.T) {
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "ATENCION") {
		t.Errorf("View() debe contener 'ATENCION'; got: %q", view)
	}
	if !strings.Contains(view, step.GetGeneratedKeyForTest()) {
		t.Errorf("View() debe contener la key generada; got: %q", view)
	}
}

// TestAPIKeyStep_Confirm_S verifica que presionar "s" avanza Done()=true.
func TestAPIKeyStep_Confirm_S(t *testing.T) {
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	sMsg := tea.KeyPressMsg{Code: 's'}
	updated, _ := step.Update(sMsg, data)
	s := updated.(*setup.APIKeyStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de presionar 's'")
	}
}

// TestAPIKeyStep_Confirm_Blocked verifica que presionar Enter sin "s" NO avanza.
func TestAPIKeyStep_Confirm_Blocked(t *testing.T) {
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.APIKeyStep)

	if s.Done() {
		t.Error("Done() debe ser false al presionar Enter sin confirmar con 's'")
	}
}

// TestAPIKeyStep_Confirm_N verifica que presionar "n" NO avanza y muestra hint.
func TestAPIKeyStep_Confirm_N(t *testing.T) {
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{Mode: "new"}
	_ = step.Init(data)

	nMsg := tea.KeyPressMsg{Code: 'n'}
	updated, _ := step.Update(nMsg, data)
	s := updated.(*setup.APIKeyStep)

	if s.Done() {
		t.Error("Done() debe ser false al presionar 'n'")
	}
}

// TestAPIKeyStep_EditMode_HasKeys verifica que en modo edit con SeedAPIKeys muestra el selector.
func TestAPIKeyStep_EditMode_HasKeys(t *testing.T) {
	existingCfg := &config.Config{
		SeedAPIKeys: []config.SeedAPIKey{
			{Name: "default", Key: "sk-abc123"},
		},
	}
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	_ = step.Init(data)

	view := step.View(data)
	if !strings.Contains(view, "Generar nueva key") {
		t.Errorf("View() en modo edit debe contener 'Generar nueva key'; got: %q", view)
	}
	if !strings.Contains(view, "Mantener la actual") {
		t.Errorf("View() en modo edit debe contener 'Mantener la actual'; got: %q", view)
	}
}

// TestAPIKeyStep_EditMode_KeepExisting verifica que seleccionar "Mantener" setea KeepExistingKey=true.
func TestAPIKeyStep_EditMode_KeepExisting(t *testing.T) {
	existingCfg := &config.Config{
		SeedAPIKeys: []config.SeedAPIKey{
			{Name: "default", Key: "sk-abc123"},
		},
	}
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	_ = step.Init(data)

	// Navegar a opcion 1 (Mantener la actual)
	step.SetAskingModeForTest(true, 1)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.APIKeyStep)

	if !s.Done() {
		t.Error("Done() debe ser true al mantener key existente")
	}
	if !data.KeepExistingKey {
		t.Error("data.KeepExistingKey debe ser true al mantener")
	}
	if data.GeneratedAPIKey != "" {
		t.Errorf("data.GeneratedAPIKey debe estar vacio al mantener; got: %q", data.GeneratedAPIKey)
	}
}

// TestAPIKeyStep_EditMode_GenerateNew verifica que seleccionar "Generar nueva" genera key y muestra el recuadro.
func TestAPIKeyStep_EditMode_GenerateNew(t *testing.T) {
	existingCfg := &config.Config{
		SeedAPIKeys: []config.SeedAPIKey{
			{Name: "default", Key: "sk-abc123"},
		},
	}
	step := &setup.APIKeyStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	_ = step.Init(data)

	// Opcion 0 = Generar nueva key (default)
	step.SetAskingModeForTest(true, 0)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.APIKeyStep)

	// No debe estar done todavia — debe mostrar la key box para confirmar
	if s.Done() {
		t.Error("Done() no debe ser true inmediatamente; debe mostrar recuadro para confirmar")
	}

	key := s.GetGeneratedKeyForTest()
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("key generada debe empezar con 'sk-'; got: %q", key)
	}
	if data.GeneratedAPIKey == "" {
		t.Error("data.GeneratedAPIKey debe tener la key generada")
	}

	// View debe mostrar el recuadro con la key
	view := s.View(data)
	if !strings.Contains(view, "ATENCION") {
		t.Errorf("View() despues de generar nueva key debe contener 'ATENCION'; got: %q", view)
	}
}
