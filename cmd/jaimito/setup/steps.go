package setup

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// WelcomeStep es la pantalla de bienvenida del wizard.
// Muestra el banner ASCII y la lista de lo que se va a configurar.
type WelcomeStep struct {
	done bool
}

// Init implementa Step. No hay operaciones async en la bienvenida.
func (s *WelcomeStep) Init(data *SetupData) tea.Cmd {
	return nil
}

// Update implementa Step. Enter avanza al siguiente step.
func (s *WelcomeStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "enter":
			s.done = true
		}
	}
	return s, nil
}

// View implementa Step. Renderiza el banner con marco doble ASCII y la lista
// de lo que el wizard va a configurar.
func (s *WelcomeStep) View(data *SetupData) string {
	banner := TitleStyle.Render(
		"╔══════════════════════════════════════════════════════╗\n" +
			"║  jaimito — Asistente de configuracion               ║\n" +
			"╠══════════════════════════════════════════════════════╣\n" +
			"║  VPS push notification hub                           ║\n" +
			"╚══════════════════════════════════════════════════════╝",
	)

	body := "\nEste asistente te va a guiar para configurar:\n\n" +
		"  - Bot de Telegram (token y validacion)\n" +
		"  - Canales de notificacion (general + extras)\n" +
		"  - Servidor y base de datos\n" +
		"  - API key para autenticacion\n"

	hint := "\n" + HintStyle.Render("Presiona Enter para comenzar")

	return banner + body + hint
}

// Done implementa Step.
func (s *WelcomeStep) Done() bool {
	return s.done
}

// DetectConfigStep detecta el estado del config existente y ofrece opciones
// apropiadas: Editar / Crear desde cero / Cancelar (si valido) o
// Crear desde cero / Cancelar (si invalido). Si no hay config, se skipea.
type DetectConfigStep struct {
	selectedOption int
	options        []string
	done           bool
	isValid        bool // config existe y es valido
	isInvalid      bool // config existe pero invalido
}

// Init implementa Step.
// Si el config no existe (ConfigExists=false), setea Mode="new" y done=true (skip).
func (s *DetectConfigStep) Init(data *SetupData) tea.Cmd {
	if !data.ConfigExists {
		data.Mode = "new"
		s.done = true
		return nil
	}
	if data.ExistingCfg != nil {
		s.isValid = true
		s.options = []string{"Editar configuracion existente", "Crear desde cero", "Cancelar"}
	} else if data.ConfigErr != nil {
		s.isInvalid = true
		s.options = []string{"Crear desde cero", "Cancelar"}
	}
	return nil
}

// Update implementa Step.
func (s *DetectConfigStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	switch kp.String() {
	case "up", "k":
		if s.selectedOption > 0 {
			s.selectedOption--
		} else {
			s.selectedOption = len(s.options) - 1
		}
	case "down", "j":
		if s.selectedOption < len(s.options)-1 {
			s.selectedOption++
		} else {
			s.selectedOption = 0
		}
	case "enter":
		return s.selectOption(data)
	}
	return s, nil
}

// selectOption ejecuta la opcion seleccionada y retorna el nuevo estado.
func (s *DetectConfigStep) selectOption(data *SetupData) (Step, tea.Cmd) {
	selected := s.options[s.selectedOption]

	switch {
	case strings.Contains(selected, "Editar"):
		data.Mode = "edit"
		s.done = true
		return s, nil

	case strings.Contains(selected, "Crear desde cero"):
		data.Mode = "fresh"
		// Backup automatico del config actual
		if data.ConfigPath != "" {
			_ = backupConfig(data.ConfigPath) // ignorar error de backup en el flow
		}
		s.done = true
		return s, nil

	case strings.Contains(selected, "Cancelar"):
		return s, tea.Quit
	}

	return s, nil
}

// View implementa Step.
func (s *DetectConfigStep) View(data *SetupData) string {
	var sb strings.Builder

	if s.isValid && data.ExistingCfg != nil {
		sb.WriteString(TitleStyle.Render("Configuracion existente detectada"))
		sb.WriteString("\n\n")
		// Resumen compacto
		token := obfuscateToken(data.ExistingCfg.Telegram.Token)
		sb.WriteString(fmt.Sprintf("  Bot token:  %s\n", token))
		sb.WriteString(fmt.Sprintf("  Canales:    %d configurados\n", len(data.ExistingCfg.Channels)))
		sb.WriteString(fmt.Sprintf("  Servidor:   %s\n", data.ExistingCfg.Server.Listen))
		sb.WriteString(fmt.Sprintf("  Base datos: %s\n", data.ExistingCfg.Database.Path))
		sb.WriteString("\n")
	} else if s.isInvalid {
		sb.WriteString(ErrorStyle.Render("Configuracion existente con errores"))
		sb.WriteString("\n\n")
		if data.ConfigErr != nil {
			sb.WriteString(ErrorStyle.Render("  Error: " + data.ConfigErr.Error()))
		}
		sb.WriteString("\n\n")
	}

	// Opciones
	for i, opt := range s.options {
		if i == s.selectedOption {
			sb.WriteString(StepActive.Render("> " + opt))
		} else {
			sb.WriteString(HintStyle.Render("  " + opt))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(HintStyle.Render("↑/↓: mover │ Enter: seleccionar"))
	sb.WriteString("\n")

	return sb.String()
}

// Done implementa Step.
func (s *DetectConfigStep) Done() bool {
	return s.done
}

// obfuscateToken ofusca un token de Telegram mostrando solo los ultimos 6 caracteres.
// Formato esperado: "****:XXXXXX" donde XXXXXX son los ultimos 6 chars del token.
func obfuscateToken(token string) string {
	if len(token) <= 6 {
		return "****"
	}
	return "****:" + token[len(token)-6:]
}

// backupConfig copia el archivo en path a path+".bak".
func backupConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backupConfig: leer %s: %w", path, err)
	}
	bakPath := path + ".bak"
	if err := os.WriteFile(bakPath, data, 0o600); err != nil {
		return fmt.Errorf("backupConfig: escribir %s: %w", bakPath, err)
	}
	return nil
}

// PlaceholderStep es un step temporal para los steps 2-7 que se implementan
// en planes y fases posteriores. Solo muestra "Proximamente..." y avanza con Enter.
type PlaceholderStep struct {
	name string
	done bool
}

// Init implementa Step.
func (s *PlaceholderStep) Init(data *SetupData) tea.Cmd {
	return nil
}

// Update implementa Step. Enter marca done=true.
func (s *PlaceholderStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		if kp.String() == "enter" {
			s.done = true
		}
	}
	return s, nil
}

// View implementa Step.
func (s *PlaceholderStep) View(data *SetupData) string {
	return HintStyle.Render(s.name+": Proximamente...") + "\n\n" +
		HintStyle.Render("Presiona Enter para continuar")
}

// Done implementa Step.
func (s *PlaceholderStep) Done() bool {
	return s.done
}
