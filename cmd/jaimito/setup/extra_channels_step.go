package setup

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/internal/config"
)

// extraChannelState representa el estado interno de la maquina de estados del ExtraChannelsStep.
type extraChannelState int

const (
	stateAskAdd        extraChannelState = iota // "Agregar canal extra? (Si/No)"
	stateInputName                              // tipear nombre del canal
	stateInputChatID                            // tipear chat ID
	stateSelectPriority                         // elegir low/normal/high
	stateValidating                             // spinner mientras valida
	stateConfirmMore                            // "Agregar otro? (Si/No)"
)

// Constantes exportadas para tests (mapean a los estados internos).
const (
	StateAskAdd        = stateAskAdd
	StateInputName     = stateInputName
	StateInputChatID   = stateInputChatID
	StateSelectPriority = stateSelectPriority
	StateValidating    = stateValidating
	StateConfirmMore   = stateConfirmMore
)

// nameRegex valida nombres de canales: solo letras minusculas, numeros y guiones.
var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// ExtraChannelsStep es el step del wizard donde el operador configura los canales extra.
// Permite agregar 0 o mas canales con nombre, chat ID y prioridad.
type ExtraChannelsStep struct {
	state         extraChannelState
	nameInput     textinput.Model
	chatIDInput   textinput.Model
	spinner       spinner.Model
	validationSeq int
	validError    string

	// Canal en progreso (variables temporales per Pitfall 6)
	currentName     string
	currentChatID   int64
	currentPriority int // indice en priorities slice (0=low, 1=normal, 2=high)

	// Canales extra acumulados (sin incluir "general")
	extraChannels []config.ChannelConfig

	// Selector Si/No (0=Si, 1=No)
	confirmOption int

	// Prioridades disponibles
	priorities []string

	// Chat validado info (para mostrar en stateConfirmMore)
	chatTitle string
	chatType  string

	// Modo edit
	editChannels []config.ChannelConfig

	done bool
}

// Init implementa Step. Resetea estado completo para ser idempotente ante re-entry.
func (s *ExtraChannelsStep) Init(data *SetupData) tea.Cmd {
	// Reset completo: Init puede ser llamado multiples veces por navegacion Esc/Enter
	s.extraChannels = nil
	s.editChannels = nil
	s.done = false
	s.validError = ""
	s.confirmOption = 0
	s.chatTitle = ""
	s.chatType = ""
	s.currentName = ""
	s.currentChatID = 0
	s.currentPriority = 1 // default "normal" per D-11
	s.validationSeq = 0

	s.nameInput = textinput.New()
	s.nameInput.Placeholder = "deploys"
	s.nameInput.CharLimit = 30

	s.chatIDInput = textinput.New()
	s.chatIDInput.Placeholder = "-1001234567890"
	s.chatIDInput.CharLimit = 20

	s.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))

	s.priorities = []string{"low", "normal", "high"}

	// Modo edit: pre-cargar canales extra (todos menos "general")
	if data.Mode == "edit" && data.ExistingCfg != nil {
		for _, ch := range data.ExistingCfg.Channels {
			if ch.Name != "general" {
				s.editChannels = append(s.editChannels, ch)
				s.extraChannels = append(s.extraChannels, ch)
			}
		}
		if len(s.editChannels) > 0 {
			// Hay canales pre-cargados: ir a stateConfirmMore para ver y decidir agregar mas
			s.state = stateConfirmMore
			return nil
		}
	}

	s.state = stateAskAdd
	return nil
}

// Update implementa Step. Maquina de estados para el flujo de canales extra.
func (s *ExtraChannelsStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if s.state == stateValidating {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil

	case chatValidationResultMsg:
		// Descartar respuestas stale
		if msg.seq != s.validationSeq {
			return s, nil
		}
		s.state = stateConfirmMore // reset en cualquier caso
		if msg.err != nil {
			s.validError = msg.err.Error()
			s.state = stateInputChatID
			return s, s.chatIDInput.Focus()
		}
		// Exito: crear canal y agregar a extraChannels
		s.chatTitle = msg.chatTitle
		s.chatType = msg.chatType
		s.extraChannels = append(s.extraChannels, config.ChannelConfig{
			Name:     s.currentName,
			ChatID:   msg.chatID,
			Priority: s.priorities[s.currentPriority],
		})
		s.validError = ""
		s.confirmOption = 0 // reset a "Si" para la confirmacion
		s.state = stateConfirmMore
		return s, nil

	case tea.KeyPressMsg:
		return s.handleKey(msg, data)
	}

	return s, nil
}

// handleKey maneja los eventos de teclado segun el estado actual.
func (s *ExtraChannelsStep) handleKey(msg tea.KeyPressMsg, data *SetupData) (Step, tea.Cmd) {
	switch s.state {
	case stateAskAdd, stateConfirmMore:
		return s.handleConfirmKey(msg, data)

	case stateInputName:
		return s.handleNameKey(msg, data)

	case stateInputChatID:
		return s.handleChatIDKey(msg, data)

	case stateSelectPriority:
		return s.handlePriorityKey(msg, data)

	case stateValidating:
		// Ignorar input durante validacion
		return s, nil
	}

	return s, nil
}

// handleConfirmKey maneja los estados stateAskAdd y stateConfirmMore (Si/No).
func (s *ExtraChannelsStep) handleConfirmKey(msg tea.KeyPressMsg, data *SetupData) (Step, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if s.confirmOption > 0 {
			s.confirmOption--
		}
	case "down", "j":
		if s.confirmOption < 1 {
			s.confirmOption++
		}
	case "enter":
		if s.confirmOption == 0 {
			// Si: transicionar a ingresar nombre
			s.validError = ""
			s.nameInput.SetValue("")
			s.state = stateInputName
			return s, s.nameInput.Focus()
		}
		// No: commitear canales extra a data.Channels y marcar done
		data.Channels = append(data.Channels, s.extraChannels...)
		s.done = true
		return s, nil
	}
	return s, nil
}

// handleNameKey maneja el estado stateInputName.
func (s *ExtraChannelsStep) handleNameKey(msg tea.KeyPressMsg, data *SetupData) (Step, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(s.nameInput.Value())
		if name == "" {
			s.validError = "el nombre no puede estar vacio"
			return s, nil
		}
		if !nameRegex.MatchString(name) {
			s.validError = fmt.Sprintf("nombre invalido: solo letras minusculas, numeros y guiones (^[a-z0-9-]+$)")
			return s, nil
		}
		// Verificar que no sea duplicado con "general" ni con extraChannels
		if name == "general" {
			s.validError = fmt.Sprintf("el nombre %q ya existe (canal obligatorio)", name)
			return s, nil
		}
		for _, ch := range s.extraChannels {
			if ch.Name == name {
				s.validError = fmt.Sprintf("el nombre %q ya existe como canal extra", name)
				return s, nil
			}
		}
		// Nombre valido: transicionar a ingresar chat ID
		s.currentName = name
		s.validError = ""
		s.chatIDInput.SetValue("")
		s.state = stateInputChatID
		return s, s.chatIDInput.Focus()

	default:
		var cmd tea.Cmd
		s.nameInput, cmd = s.nameInput.Update(msg)
		return s, cmd
	}
}

// handleChatIDKey maneja el estado stateInputChatID.
func (s *ExtraChannelsStep) handleChatIDKey(msg tea.KeyPressMsg, data *SetupData) (Step, tea.Cmd) {
	switch msg.String() {
	case "enter":
		chatID, err := parseChatID(s.chatIDInput.Value())
		if err != nil {
			s.validError = err.Error()
			return s, nil
		}
		// Chat ID valido: transicionar a seleccionar prioridad
		s.currentChatID = chatID
		s.validError = ""
		s.state = stateSelectPriority
		return s, nil

	default:
		var cmd tea.Cmd
		s.chatIDInput, cmd = s.chatIDInput.Update(msg)
		return s, cmd
	}
}

// handlePriorityKey maneja el estado stateSelectPriority.
func (s *ExtraChannelsStep) handlePriorityKey(msg tea.KeyPressMsg, data *SetupData) (Step, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if s.currentPriority > 0 {
			s.currentPriority--
		}
	case "down", "j":
		if s.currentPriority < len(s.priorities)-1 {
			s.currentPriority++
		}
	case "enter":
		// Transicionar a validando
		s.validationSeq++
		s.state = stateValidating
		s.validError = ""
		return s, tea.Batch(
			validateChatCmd(data.ValidatedBot, s.currentChatID, s.validationSeq),
			s.spinner.Tick,
		)
	}
	return s, nil
}

// View implementa Step. Renderiza el estado actual.
func (s *ExtraChannelsStep) View(data *SetupData) string {
	var sb strings.Builder

	// Titulo
	sb.WriteString(TitleStyle.Render("Canales Extra"))
	sb.WriteString("\n\n")

	// Mostrar canales extra ya agregados
	if len(s.extraChannels) > 0 {
		sb.WriteString("Canales configurados:\n")
		for _, ch := range s.extraChannels {
			sb.WriteString(fmt.Sprintf("  %s → chat_id: %d [%s]\n", ch.Name, ch.ChatID, ch.Priority))
		}
		sb.WriteString("\n")
	}

	switch s.state {
	case stateAskAdd:
		sb.WriteString("Agregar un canal extra? (opcional)\n\n")
		sb.WriteString(renderConfirmSelector(s.confirmOption))
		sb.WriteString("\n")
		sb.WriteString(HintStyle.Render("Ejemplos: deploys, errors, cron, monitoring"))
		sb.WriteString("\n")

	case stateInputName:
		sb.WriteString("Nombre del canal:\n")
		sb.WriteString(s.nameInput.View())
		sb.WriteString("\n")
		if s.validError != "" {
			sb.WriteString(ErrorStyle.Render(s.validError))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(HintStyle.Render("Solo letras minusculas, numeros y guiones"))
		sb.WriteString("\n")

	case stateInputChatID:
		sb.WriteString(fmt.Sprintf("Chat ID para '%s':\n", s.currentName))
		sb.WriteString(s.chatIDInput.View())
		sb.WriteString("\n")
		if s.validError != "" {
			sb.WriteString(ErrorStyle.Render(s.validError))
			sb.WriteString("\n")
		}

	case stateSelectPriority:
		sb.WriteString(fmt.Sprintf("Prioridad para '%s':\n\n", s.currentName))
		for i, p := range s.priorities {
			if i == s.currentPriority {
				sb.WriteString(StepActive.Render("> " + p))
			} else {
				sb.WriteString(StepPending.Render("  " + p))
			}
			sb.WriteString("\n")
		}

	case stateValidating:
		sb.WriteString(s.spinner.View())
		sb.WriteString(" Verificando acceso al chat...\n")

	case stateConfirmMore:
		if s.validError != "" {
			sb.WriteString(ErrorStyle.Render(s.validError))
			sb.WriteString("\n\n")
		} else if len(s.extraChannels) > 0 {
			last := s.extraChannels[len(s.extraChannels)-1]
			if s.chatTitle != "" {
				sb.WriteString(StepDone.Render(fmt.Sprintf("✓ Canal '%s' → %s (%s)", last.Name, s.chatTitle, s.chatType)))
			} else {
				sb.WriteString(StepDone.Render(fmt.Sprintf("✓ Canal '%s' configurado", last.Name)))
			}
			sb.WriteString("\n\n")
		}
		sb.WriteString("Agregar otro canal?\n\n")
		sb.WriteString(renderConfirmSelector(s.confirmOption))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderConfirmSelector renderiza el selector Si/No.
func renderConfirmSelector(selected int) string {
	var sb strings.Builder
	options := []string{"Si", "No"}
	for i, opt := range options {
		if i == selected {
			sb.WriteString(StepActive.Render("> " + opt))
		} else {
			sb.WriteString(StepPending.Render("  " + opt))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// Done implementa Step.
func (s *ExtraChannelsStep) Done() bool {
	return s.done
}

// --- Metodos auxiliares para tests ---

// SetValidationState permite a los tests simular el estado de validacion.
func (s *ExtraChannelsStep) SetValidationState(seq int, validating bool) {
	s.validationSeq = seq
	if validating {
		s.state = stateValidating
	}
}

// SetStateForTest permite a los tests setear el estado interno directamente.
func (s *ExtraChannelsStep) SetStateForTest(state extraChannelState) {
	s.state = state
	// Inicializar inputs si no lo fueron
	if !s.nameInput.Focused() && state == stateInputName {
		_ = s.nameInput.Focus()
	}
	if state == stateInputChatID {
		_ = s.chatIDInput.Focus()
	}
}

// SetInputNameForTest permite a los tests setear el valor del nameInput.
func (s *ExtraChannelsStep) SetInputNameForTest(value string) {
	s.nameInput.SetValue(value)
}

// SetInputChatIDForTest permite a los tests setear el valor del chatIDInput.
func (s *ExtraChannelsStep) SetInputChatIDForTest(value string) {
	s.chatIDInput.SetValue(value)
}

// SetCurrentNameForTest permite a los tests setear el currentName.
func (s *ExtraChannelsStep) SetCurrentNameForTest(name string) {
	s.currentName = name
}

// GetCurrentPriorityForTest retorna el indice de prioridad actual para tests.
func (s *ExtraChannelsStep) GetCurrentPriorityForTest() int {
	return s.currentPriority
}

// AddExtraChannelForTest agrega un canal extra directamente para tests.
func (s *ExtraChannelsStep) AddExtraChannelForTest(ch config.ChannelConfig) {
	s.extraChannels = append(s.extraChannels, ch)
	// Si ya habia canales, estar en stateConfirmMore
	s.state = stateConfirmMore
}
