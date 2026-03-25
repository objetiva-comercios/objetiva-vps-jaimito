package setup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/go-telegram/bot"

	"github.com/chiguire/jaimito/internal/config"
)

// chatValidationResultMsg es el mensaje de resultado de la validacion async de un chat ID.
type chatValidationResultMsg struct {
	seq       int
	chatTitle string
	chatType  string // "group", "supergroup", "channel", "private"
	chatID    int64
	err       error
}

// ChatValidationResultMsg es la version exportada para tests.
type ChatValidationResultMsg = chatValidationResultMsg

// NewChatValidationResultMsg crea un chatValidationResultMsg para uso en tests.
func NewChatValidationResultMsg(seq int, chatID int64, chatTitle, chatType string, err error) chatValidationResultMsg {
	return chatValidationResultMsg{
		seq:       seq,
		chatID:    chatID,
		chatTitle: chatTitle,
		chatType:  chatType,
		err:       err,
	}
}

// validateChatCmd es un tea.Cmd que llama bot.GetChat con timeout de 10 segundos.
// Captura seq en el closure para descartar respuestas stale.
func validateChatCmd(b *bot.Bot, chatID int64, seq int) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				// Retornar error seguro en lugar de crash (Pitfall W5)
				_ = r
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		chat, err := b.GetChat(ctx, &bot.GetChatParams{ChatID: chatID})
		if err != nil {
			return chatValidationResultMsg{seq: seq, chatID: chatID, err: err}
		}
		return chatValidationResultMsg{
			seq:       seq,
			chatID:    chatID,
			chatTitle: chat.Title,
			chatType:  string(chat.Type),
		}
	}
}

// parseChatID parsea un string a int64, retornando error descriptivo si el formato es invalido.
func parseChatID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("ingresa un chat ID")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("chat ID invalido: debe ser numerico (ej: -1001234567890)")
	}
	return id, nil
}

// GeneralChannelStep es el step del wizard donde el operador configura el canal general.
// Valida el chat ID en vivo contra la API de Telegram usando el bot ya validado.
type GeneralChannelStep struct {
	input          textinput.Model
	spinner        spinner.Model
	validationSeq  int
	validating     bool
	validError     string
	chatTitle      string // titulo del chat validado
	chatType       string // tipo del chat validado
	validatedID    int64  // chat ID validado
	originalChatID int64  // para modo edit: detectar si cambio
	chatChanged    bool   // true si el operador modifico el input
	done           bool
}

// Init implementa Step. Configura el textinput y el spinner.
// En modo "edit" con config existente, pre-llena con el chat ID del canal general.
func (s *GeneralChannelStep) Init(data *SetupData) tea.Cmd {
	s.input = textinput.New()
	s.input.Placeholder = "-1001234567890"
	s.input.CharLimit = 20

	s.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))

	// Pre-llenar en modo edit si existe canal "general"
	if data.Mode == "edit" && data.ExistingCfg != nil {
		for _, ch := range data.ExistingCfg.Channels {
			if ch.Name == "general" {
				s.input.SetValue(strconv.FormatInt(ch.ChatID, 10))
				s.originalChatID = ch.ChatID
				s.chatChanged = false
				break
			}
		}
	}

	return s.input.Focus()
}

// Update implementa Step. Maneja eventos de teclado, resultados de validacion async, y spinner ticks.
func (s *GeneralChannelStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// CRITICO: el spinner DEBE recibir TickMsg o se detiene (Pitfall W3)
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case chatValidationResultMsg:
		// Descartar respuestas stale (Pitfall W2)
		if msg.seq != s.validationSeq {
			return s, nil
		}
		s.validating = false
		if msg.err != nil {
			s.validError = msg.err.Error()
			return s, s.input.Focus()
		}
		// Exito: guardar info del chat y agregar ChannelConfig a data.Channels
		s.done = true
		s.chatTitle = msg.chatTitle
		s.chatType = msg.chatType
		s.validatedID = msg.chatID
		data.Channels = append(data.Channels, config.ChannelConfig{
			Name:     "general",
			ChatID:   msg.chatID,
			Priority: "normal",
		})
		return s, nil

	case tea.KeyPressMsg:
		// Durante validacion: ignorar input
		if s.validating {
			return s, nil
		}

		switch msg.String() {
		case "enter":
			raw := s.input.Value()

			// Modo edit sin cambio: mantener existente sin re-validar (D-08)
			if data.Mode == "edit" && !s.chatChanged && s.originalChatID != 0 {
				s.done = true
				s.validatedID = s.originalChatID
				data.Channels = append(data.Channels, config.ChannelConfig{
					Name:     "general",
					ChatID:   s.originalChatID,
					Priority: "normal",
				})
				return s, nil
			}

			chatID, err := parseChatID(raw)
			if err != nil {
				s.validError = err.Error()
				return s, nil
			}

			// Lanzar validacion async
			s.validationSeq++
			s.validating = true
			s.validError = ""
			return s, tea.Batch(validateChatCmd(data.ValidatedBot, chatID, s.validationSeq), s.spinner.Tick)

		default:
			// Cualquier otra tecla: delegar al input y marcar como modificado
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			s.chatChanged = true
			return s, cmd
		}
	}

	// Para otros mensajes: delegar al input
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View implementa Step. Renderiza el estado actual del step.
func (s *GeneralChannelStep) View(data *SetupData) string {
	var sb strings.Builder

	if s.validating {
		// Mostrar spinner con texto descriptivo (D-14)
		sb.WriteString(s.spinner.View())
		sb.WriteString(" Verificando acceso al chat...\n")
		return sb.String()
	}

	if s.done {
		// Mostrar resultado exitoso (D-07)
		sb.WriteString(TitleStyle.Render("Canal General"))
		sb.WriteString("\n")
		sb.WriteString(StepDone.Render("✓ " + s.chatTitle + " (" + s.chatType + ")"))
		sb.WriteString("\n")
		return sb.String()
	}

	// Estado normal: mostrar input
	sb.WriteString(TitleStyle.Render("Canal General"))
	sb.WriteString("\n\n")
	sb.WriteString("Ingresa el chat ID del canal general:\n")
	sb.WriteString(s.input.View())
	sb.WriteString("\n")

	if s.validError != "" {
		sb.WriteString(ErrorStyle.Render(s.validError))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(HintStyle.Render("Manda un mensaje en el grupo y reenvialo a @RawDataBot."))
	sb.WriteString("\n")
	sb.WriteString(HintStyle.Render("Te responde con el chat ID del grupo de origen."))
	sb.WriteString("\n")

	return sb.String()
}

// Done implementa Step.
func (s *GeneralChannelStep) Done() bool {
	return s.done
}

// --- Metodos auxiliares para tests ---

// SetValidationState permite a los tests simular el estado de validacion en progreso.
func (s *GeneralChannelStep) SetValidationState(seq int, validating bool) {
	s.validationSeq = seq
	s.validating = validating
}

// SetDoneForTest permite a los tests setear el estado done directamente.
func (s *GeneralChannelStep) SetDoneForTest(done bool) {
	s.done = done
}

// SetChatInfoForTest permite a los tests setear info del chat para verificar la vista.
func (s *GeneralChannelStep) SetChatInfoForTest(title, chatType string) {
	s.chatTitle = title
	s.chatType = chatType
}

// SetInputValueForTest permite a los tests setear el valor del textinput directamente.
func (s *GeneralChannelStep) SetInputValueForTest(value string) {
	s.input.SetValue(value)
	s.chatChanged = true
}
