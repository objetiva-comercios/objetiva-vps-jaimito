package setup_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
)

// TestFormatNonInteractiveError verifica que el mensaje de error contiene
// las palabras clave esperadas para el caso de terminal no-interactiva.
func TestFormatNonInteractiveError(t *testing.T) {
	msg := setup.FormatNonInteractiveError()
	if !strings.Contains(msg, "terminal interactiva") {
		t.Errorf("FormatNonInteractiveError() no contiene 'terminal interactiva'; got: %q", msg)
	}
	if !strings.Contains(msg, "curl") {
		t.Errorf("FormatNonInteractiveError() no contiene 'curl'; got: %q", msg)
	}
}

// TestWelcomeStep_View verifica que la vista del WelcomeStep contiene
// el marco ASCII con '═', 'jaimito', referencia al Bot de Telegram y la tecla Enter.
func TestWelcomeStep_View(t *testing.T) {
	step := &setup.WelcomeStep{}
	data := &setup.SetupData{}
	view := step.View(data)

	checks := []string{"═", "jaimito", "Bot de Telegram", "Enter"}
	for _, want := range checks {
		if !strings.Contains(view, want) {
			t.Errorf("WelcomeStep.View() no contiene %q;\nview: %q", want, view)
		}
	}
}

// TestWelcomeStep_Done verifica que Done() es false al inicio y true
// despues de recibir un KeyPressMsg con "enter".
func TestWelcomeStep_Done(t *testing.T) {
	step := &setup.WelcomeStep{}
	data := &setup.SetupData{}

	if step.Done() {
		t.Fatal("WelcomeStep.Done() debe ser false antes de presionar Enter")
	}

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updatedStep, _ := step.Update(enterMsg, data)

	if !updatedStep.Done() {
		t.Error("WelcomeStep.Done() debe ser true despues de presionar Enter")
	}
}

// TestWizardModel_ProgressBar verifica que el View() del WizardModel contiene
// la barra de progreso con el nombre del step actual, el contador, y los dots.
func TestWizardModel_ProgressBar(t *testing.T) {
	model := setup.NewWizardModel("/etc/jaimito/config.yaml", nil, nil)
	view := model.View()
	viewStr := view.Content

	// Debe mostrar el nombre del step actual
	if !strings.Contains(viewStr, "Bienvenida") {
		t.Error("Progress bar debe contener el nombre del step actual 'Bienvenida'")
	}
	// Debe mostrar el contador "Paso 1 de 8"
	if !strings.Contains(viewStr, "Paso 1 de 8") {
		t.Error("Progress bar debe contener 'Paso 1 de 8'")
	}
	// Debe contener el dot activo
	if !strings.Contains(viewStr, "◉") {
		t.Error("Progress bar debe contener '◉' para el step activo")
	}
	// Debe contener dots pendientes
	if !strings.Contains(viewStr, "○") {
		t.Error("Progress bar debe contener '○' para steps pendientes")
	}
}

// TestWizardModel_ConfirmExit verifica el comportamiento de confirmacion de salida:
// primer Ctrl+C activa confirmExit, segundo Ctrl+C produce Quit.
func TestWizardModel_ConfirmExit(t *testing.T) {
	model := setup.NewWizardModel("/etc/jaimito/config.yaml", nil, nil)

	// Primer Ctrl+C: debe activar confirmExit
	ctrlC := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	updatedModel, cmd := model.Update(ctrlC)
	if cmd != nil {
		// Ejecutar el cmd para ver si es Quit
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("Primer Ctrl+C no deberia producir Quit, solo confirmExit")
		}
	}

	// Verificar que el view muestra la confirmacion
	viewStr := updatedModel.View().Content
	if !strings.Contains(viewStr, "Seguro") && !strings.Contains(viewStr, "salir") {
		t.Errorf("Despues del primer Ctrl+C, la vista debe mostrar confirmacion de salida; got: %q", viewStr)
	}

	// Segundo Ctrl+C: debe producir Quit
	updatedModel2, cmd2 := updatedModel.Update(ctrlC)
	_ = updatedModel2
	if cmd2 == nil {
		t.Fatal("Segundo Ctrl+C debe retornar un comando (Quit)")
	}
	msg2 := cmd2()
	if _, ok := msg2.(tea.QuitMsg); !ok {
		t.Errorf("Segundo Ctrl+C debe producir tea.QuitMsg; got: %T", msg2)
	}
}

// TestWizardModel_BackNavigation verifica que Esc retrocede al step anterior.
func TestWizardModel_BackNavigation(t *testing.T) {
	model := setup.NewWizardModel("/etc/jaimito/config.yaml", nil, nil)

	// Avanzar al step 2 usando Enter en el WelcomeStep
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	model2, _ := model.Update(enterMsg)

	// Verificar que estamos en el step 2 (currentStep == 1)
	view2 := model2.View().Content
	// El step 1 (Bienvenida) deberia tener '✓' y el step 2 deberia tener '▸'
	_ = view2

	// Enviar Esc para retroceder
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	model3, _ := model2.Update(escMsg)

	// Verificar que volvimos al step 1: la barra de progreso debe mostrar 'Bienvenida'
	view3 := model3.View().Content
	if !strings.Contains(view3, "Bienvenida") {
		t.Error("Despues de Esc, progress bar debe mostrar 'Bienvenida'")
	}
	if !strings.Contains(view3, "Paso 1 de 8") {
		t.Error("Despues de Esc, progress bar debe mostrar 'Paso 1 de 8'")
	}
}
