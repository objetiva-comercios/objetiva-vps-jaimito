package setup_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// sendKey es un helper para enviar una tecla al step.
func sendKey(t *testing.T, step setup.Step, data *setup.SetupData, key string) setup.Step {
	t.Helper()
	var msg tea.Msg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		// Caracteres normales - en bubbletea v2 se usan via Text field
		msg = tea.KeyPressMsg{Text: key}
	}
	updated, _ := step.Update(msg, data)
	return updated
}

// TestExtraChannelsStep_NoExtra verifica que responder "No" a "Agregar canal extra?" setea done=true.
func TestExtraChannelsStep_NoExtra(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// El step inicia en stateAskAdd preguntando "Agregar canal extra?"
	// Seleccion inicial es "Si" (0), mover a "No" con down
	updated := sendKey(t, step, data, "down")
	// Confirmar "No" con Enter
	updated = sendKey(t, updated, data, "enter")
	s := updated.(*setup.ExtraChannelsStep)

	if !s.Done() {
		t.Error("Done() debe ser true despues de responder No")
	}
}

// TestExtraChannelsStep_AddOneChannel verifica el flujo completo de agregar un canal extra.
func TestExtraChannelsStep_AddOneChannel(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{
		Mode: "new",
		// Canal general ya cargado (simulado por step anterior)
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: -1001234567890, Priority: "normal"},
		},
	}
	step.Init(data)

	// Responder "Si" a "Agregar canal extra?" (opcion default)
	updated := sendKey(t, step, data, "enter")

	// Ingresar nombre del canal: "deploys"
	s := updated.(*setup.ExtraChannelsStep)
	s.SetInputNameForTest("deploys")
	updated = sendKey(t, s, data, "enter")

	// Ingresar chat ID: -1009876543210
	s = updated.(*setup.ExtraChannelsStep)
	s.SetInputChatIDForTest("-1009876543210")
	updated = sendKey(t, s, data, "enter")

	// Seleccionar prioridad "high" (2) con 2 presses de down
	updated = sendKey(t, updated, data, "down")
	updated = sendKey(t, updated, data, "down")
	// Confirmar prioridad con Enter -> lanza validacion async, incrementa seq a 1
	updated = sendKey(t, updated, data, "enter")
	// Ahora enviar resultado de validacion exitosa con seq=1
	msg := setup.NewChatValidationResultMsg(1, -1009876543210, "Deploys Channel", "supergroup", nil)
	s = updated.(*setup.ExtraChannelsStep)
	updated, _ = s.Update(msg, data)

	// Responder "No" a "Agregar otro canal?"
	updated = sendKey(t, updated, data, "down")
	updated = sendKey(t, updated, data, "enter")

	s = updated.(*setup.ExtraChannelsStep)
	if !s.Done() {
		t.Error("Done() debe ser true despues de agregar canal y responder No")
	}

	// Verificar que data.Channels tiene el canal general + el extra
	found := false
	for _, ch := range data.Channels {
		if ch.Name == "deploys" && ch.ChatID == -1009876543210 && ch.Priority == "high" {
			found = true
		}
	}
	if !found {
		t.Errorf("data.Channels debe contener el canal 'deploys'; channels: %v", data.Channels)
	}
}

// TestExtraChannelsStep_DuplicateName verifica que el nombre "general" es rechazado (duplicado).
func TestExtraChannelsStep_DuplicateName(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{
		Mode: "new",
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: -1001234567890, Priority: "normal"},
		},
	}
	step.Init(data)

	// Responder "Si" a "Agregar canal extra?"
	updated := sendKey(t, step, data, "enter")

	// Intentar ingresar nombre "general" (duplicado)
	s := updated.(*setup.ExtraChannelsStep)
	s.SetInputNameForTest("general")
	updated = sendKey(t, s, data, "enter")

	s = updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con nombre duplicado")
	}

	view := s.View(data)
	if !isErrorInView(view) {
		t.Errorf("View() debe mostrar error de nombre duplicado; got: %q", view)
	}
}

// isErrorInView verifica si la vista contiene algun indicador de error.
func isErrorInView(view string) bool {
	// Los errores se muestran con ErrorStyle en rojo, verificamos contenido semantico
	return len(view) > 0 && (containsAny(view, "duplicado", "existe", "invalido", "solo", "letras"))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// TestExtraChannelsStep_DuplicateExtraName verifica que dos canales extra con mismo nombre son rechazados.
func TestExtraChannelsStep_DuplicateExtraName(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Agregar canal "deploys" directamente via test helper
	step.AddExtraChannelForTest(config.ChannelConfig{Name: "deploys", ChatID: -100111, Priority: "normal"})

	// Responder "Si" a "Agregar otro canal?"
	updated := sendKey(t, step, data, "enter")

	// Intentar ingresar nombre "deploys" (ya existe)
	s := updated.(*setup.ExtraChannelsStep)
	s.SetInputNameForTest("deploys")
	updated = sendKey(t, s, data, "enter")

	s = updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con nombre de canal extra duplicado")
	}
}

// TestExtraChannelsStep_InvalidName verifica que nombres invalidos son rechazados.
func TestExtraChannelsStep_InvalidName(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Responder "Si" a "Agregar canal extra?"
	updated := sendKey(t, step, data, "enter")

	// Intentar ingresar nombre con mayuscula (invalido)
	s := updated.(*setup.ExtraChannelsStep)
	s.SetInputNameForTest("Deploy")
	updated = sendKey(t, s, data, "enter")

	s = updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con nombre invalido (mayuscula)")
	}

	// Intentar nombre con espacios
	s.SetInputNameForTest("my channel")
	updated = sendKey(t, s, data, "enter")
	s = updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con nombre invalido (espacio)")
	}

	// Intentar nombre vacio
	s.SetInputNameForTest("")
	updated = sendKey(t, s, data, "enter")
	s = updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con nombre vacio")
	}
}

// TestExtraChannelsStep_PrioritySelector verifica que up/down cambia prioridad y Enter confirma.
func TestExtraChannelsStep_PrioritySelector(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Avanzar a stateSelectPriority directamente via helper
	step.SetStateForTest(setup.StateSelectPriority)

	// Verificar que la vista muestra las 3 prioridades
	view := step.View(data)
	if !containsAll(view, "low", "normal", "high") {
		t.Errorf("View() debe mostrar las 3 prioridades; got: %q", view)
	}

	// Down incrementa la seleccion (default es 1=normal, down va a 2=high)
	updated := sendKey(t, step, data, "down")
	s := updated.(*setup.ExtraChannelsStep)
	if s.GetCurrentPriorityForTest() != 2 {
		t.Errorf("CurrentPriority debe ser 2 despues de down; got: %d", s.GetCurrentPriorityForTest())
	}

	// Up decrementa (de 2=high a 1=normal)
	updated = sendKey(t, s, data, "up")
	s = updated.(*setup.ExtraChannelsStep)
	if s.GetCurrentPriorityForTest() != 1 {
		t.Errorf("CurrentPriority debe ser 1 despues de up; got: %d", s.GetCurrentPriorityForTest())
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsAny(s, sub) {
			return false
		}
	}
	return true
}

// TestExtraChannelsStep_InvalidChatID verifica que chat ID no-numerico muestra error sin lanzar validacion.
func TestExtraChannelsStep_InvalidChatID(t *testing.T) {
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{Mode: "new"}
	step.Init(data)

	// Avanzar a stateInputChatID
	step.SetStateForTest(setup.StateInputChatID)
	step.SetCurrentNameForTest("deploys")

	// Ingresar chat ID no-numerico
	step.SetInputChatIDForTest("no-es-numerico")
	updated := sendKey(t, step, data, "enter")

	s := updated.(*setup.ExtraChannelsStep)
	if s.Done() {
		t.Error("Done() debe ser false con chat ID invalido")
	}
	view := s.View(data)
	if !containsAny(view, "numerico", "invalido") {
		t.Errorf("View() debe mostrar error de formato; got: %q", view)
	}
}

// TestExtraChannelsStep_EditModePreload verifica que en modo edit se pre-cargan los canales extra existentes.
func TestExtraChannelsStep_EditModePreload(t *testing.T) {
	existingCfg := &config.Config{
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: -1001234567890, Priority: "normal"},
			{Name: "deploys", ChatID: -1009876543210, Priority: "high"},
			{Name: "errors", ChatID: -1005555555555, Priority: "normal"},
		},
	}
	step := &setup.ExtraChannelsStep{}
	data := &setup.SetupData{
		Mode:        "edit",
		ExistingCfg: existingCfg,
	}
	step.Init(data)

	// En modo edit con canales extra, debe mostrar los canales pre-cargados
	view := step.View(data)
	if !containsAny(view, "deploys", "errors") {
		t.Errorf("View() debe mostrar canales pre-cargados; got: %q", view)
	}
}
