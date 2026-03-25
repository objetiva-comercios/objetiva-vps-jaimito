package setup

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	db "github.com/chiguire/jaimito/internal/db"
)

// apiKeyBoxStyle es el estilo del recuadro que muestra la API key generada.
var apiKeyBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorYellow).
	Padding(0, 1)

// APIKeyStep es el step del wizard donde se genera y confirma la API key.
// En modo "new"/"fresh": genera key en Init, la muestra en recuadro, requiere "s" para confirmar.
// En modo "edit" con keys existentes: ofrece selector mantener/generar nueva.
type APIKeyStep struct {
	generatedKey    string
	confirmed       bool
	done            bool
	confirmError    string
	hasExistingKeys bool
	keepExisting    bool
	askingMode      bool
	selectedOption  int
	keyGenerated    bool
}

// Init implementa Step.
func (s *APIKeyStep) Init(data *SetupData) tea.Cmd {
	if data.Mode == "edit" && data.ExistingCfg != nil && len(data.ExistingCfg.SeedAPIKeys) > 0 {
		s.hasExistingKeys = true
		s.askingMode = true
		return nil
	}

	key, err := db.GenerateRawKey()
	if err != nil {
		s.confirmError = "Error generando API key: " + err.Error()
		return nil
	}
	s.generatedKey = key
	data.GeneratedAPIKey = key
	return nil
}

// Update implementa Step.
func (s *APIKeyStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	if s.askingMode {
		switch kp.String() {
		case "up", "k":
			if s.selectedOption > 0 {
				s.selectedOption--
			}
		case "down", "j":
			if s.selectedOption < 1 {
				s.selectedOption++
			}
		case "enter":
			if s.selectedOption == 1 {
				// Mantener la actual
				data.KeepExistingKey = true
				s.done = true
			} else {
				// Generar nueva key
				s.askingMode = false
				key, err := db.GenerateRawKey()
				if err != nil {
					s.confirmError = "Error generando API key: " + err.Error()
					return s, nil
				}
				s.generatedKey = key
				data.GeneratedAPIKey = key
				s.keyGenerated = true
			}
		}
		return s, nil
	}

	// Modo con key generada: esperar "s" para confirmar
	switch kp.String() {
	case "s", "S":
		s.confirmed = true
		s.done = true
	case "enter":
		s.confirmError = "Escribi 's' para confirmar que copiaste la key."
	default:
		s.confirmError = ""
	}
	return s, nil
}

// View implementa Step.
func (s *APIKeyStep) View(data *SetupData) string {
	var sb strings.Builder

	if s.askingMode {
		sb.WriteString(TitleStyle.Render("API Key"))
		sb.WriteString("\n\n")
		sb.WriteString("Ya existe una API key en la configuracion actual.\n\n")

		if s.selectedOption == 0 {
			sb.WriteString(StepActive.Render("> Generar nueva key"))
		} else {
			sb.WriteString(HintStyle.Render("  Generar nueva key"))
		}
		sb.WriteString("\n")
		if s.selectedOption == 1 {
			sb.WriteString(StepActive.Render("> Mantener la actual"))
		} else {
			sb.WriteString(HintStyle.Render("  Mantener la actual"))
		}
		sb.WriteString("\n")
		sb.WriteString(HintStyle.Render("Generar una nueva key NO revoca la anterior."))
		sb.WriteString("\n\n")
		return sb.String()
	}

	sb.WriteString(TitleStyle.Render("API Key"))
	sb.WriteString("\n\n")
	sb.WriteString("Se genero una API key unica para autenticar requests a jaimito.\n\n")
	sb.WriteString(WarningStyle.Render("ATENCION: Esta es la unica vez que vas a ver esta key. Copiarla antes de continuar."))
	sb.WriteString("\n\n")
	sb.WriteString(apiKeyBoxStyle.Render(s.generatedKey))
	sb.WriteString("\n\n")
	sb.WriteString("La copiaste? (s/n): ")
	if s.confirmError != "" {
		sb.WriteString(ErrorStyle.Render(s.confirmError))
	}
	sb.WriteString("\n\n")
	sb.WriteString(HintStyle.Render("Escribi 's' para confirmar que copiaste la key"))
	sb.WriteString("\n")

	return sb.String()
}

// Done implementa Step.
func (s *APIKeyStep) Done() bool {
	return s.done
}

// SetAskingModeForTest permite a los tests configurar el modo de seleccion.
func (s *APIKeyStep) SetAskingModeForTest(asking bool, selectedOption int) {
	s.askingMode = asking
	s.selectedOption = selectedOption
}

// GetGeneratedKeyForTest retorna la key generada para verificacion en tests.
func (s *APIKeyStep) GetGeneratedKeyForTest() string {
	return s.generatedKey
}
