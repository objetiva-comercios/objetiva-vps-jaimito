package setup

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// ServerStep es el step del wizard donde el operador configura la direccion de escucha del servidor.
// Implementa el patron confirm-with-defaults: Enter acepta el default, cualquier valor reemplaza.
type ServerStep struct {
	input        textinput.Model
	defaultValue string
	validError   string
	done         bool
}

// Init implementa Step. Configura el textinput con el valor por defecto.
// En modo "edit" con config existente, pre-llena con el valor actual.
func (s *ServerStep) Init(data *SetupData) tea.Cmd {
	s.defaultValue = "127.0.0.1:8080"
	s.input = textinput.New()
	s.input.Placeholder = s.defaultValue
	s.input.CharLimit = 50

	if data.Mode == "edit" && data.ExistingCfg != nil && data.ExistingCfg.Server.Listen != "" {
		s.input.SetValue(data.ExistingCfg.Server.Listen)
	}

	return s.input.Focus()
}

// Update implementa Step. Maneja eventos de teclado.
func (s *ServerStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			addr := strings.TrimSpace(s.input.Value())
			if addr == "" {
				addr = s.defaultValue
			}

			if err := validateListenAddress(addr); err != nil {
				s.validError = err.Error()
				return s, nil
			}

			data.ServerListen = addr
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

// validateListenAddress valida que addr tenga formato host:puerto valido y puerto en rango 1-65535.
func validateListenAddress(addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fmt.Errorf("Formato invalido: debe ser host:puerto (ej: 127.0.0.1:8080)")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("Puerto fuera de rango: debe ser entre 1 y 65535")
	}
	return nil
}

// View implementa Step.
func (s *ServerStep) View(data *SetupData) string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("Servidor"))
	sb.WriteString("\n\n")
	currentVal := strings.TrimSpace(s.input.Value())
	if currentVal != "" && currentVal != s.defaultValue {
		sb.WriteString(StepDone.Render("Valor actual: "+currentVal))
		sb.WriteString("\n")
		sb.WriteString(HintStyle.Render("Enter para mantener, o escribi una nueva direccion:"))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Direccion donde jaimito va a escuchar conexiones HTTP.\n\n")
		sb.WriteString(fmt.Sprintf("Por defecto: %s\n", s.defaultValue))
		sb.WriteString("Enter para aceptar, o escribi una nueva direccion:\n")
	}
	sb.WriteString(s.input.View())
	sb.WriteString("\n")

	if s.validError != "" {
		sb.WriteString(ErrorStyle.Render(s.validError))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Done implementa Step.
func (s *ServerStep) Done() bool {
	return s.done
}

// SetInputValueForTest permite a los tests setear el valor del textinput directamente.
func (s *ServerStep) SetInputValueForTest(value string) {
	s.input.SetValue(value)
}
