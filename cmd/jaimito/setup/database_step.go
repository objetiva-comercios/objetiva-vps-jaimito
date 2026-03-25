package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// DatabaseStep es el step del wizard donde el operador configura la ruta del archivo SQLite.
// Implementa el patron confirm-with-defaults: Enter acepta el default, cualquier valor reemplaza.
// Si el directorio padre no existe, muestra un warning pero NO bloquea el avance.
type DatabaseStep struct {
	input        textinput.Model
	defaultValue string
	validError   string
	warning      string
	done         bool
}

// Init implementa Step. Configura el textinput con el valor por defecto.
// En modo "edit" con config existente, pre-llena con la ruta actual.
func (s *DatabaseStep) Init(data *SetupData) tea.Cmd {
	s.defaultValue = "/var/lib/jaimito/jaimito.db"
	s.input = textinput.New()
	s.input.Placeholder = s.defaultValue
	s.input.CharLimit = 200

	if data.Mode == "edit" && data.ExistingCfg != nil && data.ExistingCfg.Database.Path != "" {
		s.input.SetValue(data.ExistingCfg.Database.Path)
	}

	return s.input.Focus()
}

// Update implementa Step. Maneja eventos de teclado.
func (s *DatabaseStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			path := strings.TrimSpace(s.input.Value())
			if path == "" {
				path = s.defaultValue
			}

			// Chequear que el directorio padre existe — warning, no error
			dir := filepath.Dir(path)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				s.warning = fmt.Sprintf("El directorio %s no existe. Se creara al iniciar jaimito.", dir)
			} else {
				s.warning = ""
			}

			data.DatabasePath = path
			s.done = true
			return s, nil

		default:
			// Limpiar error al tipear
			s.validError = ""
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View implementa Step.
func (s *DatabaseStep) View(data *SetupData) string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("Base de Datos"))
	sb.WriteString("\n\n")
	currentVal := strings.TrimSpace(s.input.Value())
	if currentVal != "" && currentVal != s.defaultValue {
		sb.WriteString(StepDone.Render("Valor actual: "+currentVal))
		sb.WriteString("\n")
		sb.WriteString(HintStyle.Render("Enter para mantener, o escribi una nueva ruta:"))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Ruta del archivo SQLite donde jaimito guarda los mensajes.\n\n")
		sb.WriteString(fmt.Sprintf("Por defecto: %s\n", s.defaultValue))
		sb.WriteString("Enter para aceptar, o escribi una nueva ruta:\n")
	}
	sb.WriteString(s.input.View())
	sb.WriteString("\n")

	if s.warning != "" {
		sb.WriteString(WarningStyle.Render(s.warning))
		sb.WriteString("\n")
	}

	if s.validError != "" {
		sb.WriteString(ErrorStyle.Render(s.validError))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(HintStyle.Render("Enter: confirmar  \u2502  Esc: volver  \u2502  Ctrl+C: salir"))
	sb.WriteString("\n")

	return sb.String()
}

// Done implementa Step.
func (s *DatabaseStep) Done() bool {
	return s.done
}

// SetInputValueForTest permite a los tests setear el valor del textinput directamente.
func (s *DatabaseStep) SetInputValueForTest(value string) {
	s.input.SetValue(value)
}
