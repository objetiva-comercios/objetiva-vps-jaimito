package setup

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/go-telegram/bot"

	"github.com/chiguire/jaimito/internal/telegram"
)

// tokenValidationResultMsg es el mensaje de resultado de la validacion async del token.
type tokenValidationResultMsg struct {
	seq     int
	botInst *bot.Bot
	info    telegram.BotInfo
	err     error
}

// TokenValidationResultMsg es la version exportada para tests.
// Wrappea el tipo interno para que los tests externos puedan crear mensajes.
type TokenValidationResultMsg = tokenValidationResultMsg

// NewTokenValidationResultMsg crea un tokenValidationResultMsg para uso en tests.
func NewTokenValidationResultMsg(seq int, info telegram.BotInfo, err error) tokenValidationResultMsg {
	return tokenValidationResultMsg{seq: seq, info: info, err: err}
}

// validateTokenCmd es un tea.Cmd que llama ValidateTokenWithInfo con timeout de 10 segundos.
// Captura seq en el closure para descartar respuestas stale.
func validateTokenCmd(token string, seq int) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				// Retornar error seguro en lugar de crash (Pitfall W5)
				_ = r
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		b, info, err := telegram.ValidateTokenWithInfo(ctx, token)
		if err != nil {
			return tokenValidationResultMsg{seq: seq, err: err}
		}
		return tokenValidationResultMsg{seq: seq, botInst: b, info: info}
	}
}

// BotTokenStep es el step del wizard donde el operador ingresa y valida el bot token.
// Implementa la interface Step del wizard.
type BotTokenStep struct {
	input         textinput.Model
	spinner       spinner.Model
	validationSeq int
	validating    bool
	validError    string
	resolvedToken string // token real a usar (para modo edit: el token original)
	tokenChanged  bool   // true si el operador modifico el input
	done          bool
}

// Init implementa Step. Resetea estado completo para ser idempotente ante re-entry.
// En modo "edit" con config existente, pre-llena el input con el token ofuscado.
func (s *BotTokenStep) Init(data *SetupData) tea.Cmd {
	s.done = false
	s.validating = false
	s.validError = ""
	s.validationSeq = 0
	s.resolvedToken = ""
	s.tokenChanged = false

	s.input = textinput.New()
	s.input.Placeholder = "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
	s.input.CharLimit = 256

	s.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))

	if data.Mode == "edit" && data.ExistingCfg != nil {
		s.input.SetValue(obfuscateToken(data.ExistingCfg.Telegram.Token))
		s.tokenChanged = false
	}

	return s.input.Focus()
}

// Update implementa Step. Maneja eventos de teclado, resultados de validacion async, y spinner ticks.
func (s *BotTokenStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// CRITICO: el spinner DEBE recibir TickMsg o se detiene (Pitfall W3)
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case tokenValidationResultMsg:
		// Descartar respuestas stale (Pitfall W2)
		if msg.seq != s.validationSeq {
			return s, nil
		}
		s.validating = false
		if msg.err != nil {
			s.validError = msg.err.Error()
			return s, s.input.Focus()
		}
		// Exito: guardar datos del bot en SetupData
		s.done = true
		data.BotToken = s.resolvedToken
		data.BotUsername = msg.info.Username
		data.BotDisplayName = msg.info.DisplayName
		data.ValidatedBot = msg.botInst
		return s, nil

	case tea.KeyPressMsg:
		// Durante validacion: ignorar input (D-14: el spinner reemplaza el input)
		if s.validating {
			return s, nil
		}

		switch msg.String() {
		case "enter":
			token := strings.TrimSpace(s.input.Value())
			if token == "" {
				s.validError = "Ingresa un bot token"
				return s, nil
			}
			// Modo edit sin cambio de token: usar el token original sin re-validar (D-04, Pitfall 5)
			if data.Mode == "edit" && !s.tokenChanged && data.ExistingCfg != nil {
				data.BotToken = data.ExistingCfg.Telegram.Token
				// Copiar datos existentes si los hay (pueden no estar disponibles en edit sin re-validar)
				data.BotUsername = data.BotUsername
				data.BotDisplayName = data.BotDisplayName
				s.done = true
				return s, nil
			}
			// Iniciar validacion async
			s.validationSeq++
			s.validating = true
			s.validError = ""
			s.resolvedToken = token
			return s, tea.Batch(validateTokenCmd(token, s.validationSeq), s.spinner.Tick)

		default:
			// Cualquier otra tecla: delegar al input y marcar como modificado
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			s.tokenChanged = true
			return s, cmd
		}
	}

	// Para otros mensajes: delegar al input
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View implementa Step. Renderiza el estado actual del step.
func (s *BotTokenStep) View(data *SetupData) string {
	var sb strings.Builder

	if s.validating {
		// Mostrar spinner con texto descriptivo (D-14)
		sb.WriteString(s.spinner.View())
		sb.WriteString(" Validando bot token...\n")
		return sb.String()
	}

	if s.done {
		// Mostrar resultado exitoso (D-02)
		sb.WriteString(TitleStyle.Render("Bot Token"))
		sb.WriteString("\n")
		sb.WriteString(StepDone.Render("✓ @" + data.BotUsername + " (" + data.BotDisplayName + ")"))
		sb.WriteString("\n")
		return sb.String()
	}

	// Estado normal: mostrar input
	sb.WriteString(TitleStyle.Render("Bot Token"))
	sb.WriteString("\n\n")
	sb.WriteString("Pega el token de tu bot de Telegram:\n")
	sb.WriteString(s.input.View())
	sb.WriteString("\n")

	if s.validError != "" {
		sb.WriteString(ErrorStyle.Render(s.validError))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(HintStyle.Render("Obtene el token de @BotFather en Telegram"))
	sb.WriteString("\n")

	return sb.String()
}

// Done implementa Step.
func (s *BotTokenStep) Done() bool {
	return s.done
}

// --- Metodos auxiliares para tests (permiten manipular estado interno desde tests externos) ---

// SetValidationState permite a los tests simular el estado de validacion en progreso.
func (s *BotTokenStep) SetValidationState(seq int, validating bool) {
	s.validationSeq = seq
	s.validating = validating
}

// SetDoneForTest permite a los tests setear el estado done directamente.
func (s *BotTokenStep) SetDoneForTest(done bool) {
	s.done = done
}

// SetValidErrorForTest permite a los tests setear un error de validacion directamente.
func (s *BotTokenStep) SetValidErrorForTest(err string) {
	s.validError = err
}
