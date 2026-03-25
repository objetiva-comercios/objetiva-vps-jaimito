package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/go-telegram/bot"
	"gopkg.in/yaml.v3"

	"github.com/chiguire/jaimito/internal/config"
)

// testNotificationResultMsg es el resultado de la notificacion de test async post-escritura.
type testNotificationResultMsg struct {
	err error
}

// TestNotificationResultMsg es la version exportada para tests.
type TestNotificationResultMsg = testNotificationResultMsg

// NewTestNotificationResultMsg crea un testNotificationResultMsg para uso en tests.
func NewTestNotificationResultMsg(err error) testNotificationResultMsg {
	return testNotificationResultMsg{err: err}
}

// sendTestNotificationCmd envia un mensaje de test a Telegram con timeout de 10s.
// Defensive: si el bot es nil retorna error limpio (no panic).
func sendTestNotificationCmd(b *bot.Bot, chatID int64, hostname string) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				_ = r
			}
		}()
		if b == nil {
			return testNotificationResultMsg{err: fmt.Errorf("bot no disponible")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "\u2705 jaimito configurado correctamente en " + hostname,
		})
		return testNotificationResultMsg{err: err}
	}
}

// SummaryStep es la pantalla final del wizard: muestra el resumen completo de la
// configuracion, valida con config.Validate(), y escribe el YAML a disco.
// Fase 7: despues de escribir, envia una notificacion de test a Telegram.
type SummaryStep struct {
	confirmed   bool   // operador presiono Enter
	writeErr    string // error de escritura
	validateErr string // error de config.Validate()
	writtenPath string // path donde se escribio el config
	done        bool

	// Phase 7: notificacion de test
	spinner spinner.Model
	sending bool   // true mientras se envia la notificacion de test
	testOk  bool   // true si la notificacion fue exitosa
	testErr string // mensaje de error si fallo (no bloqueante)
}

// IsSending retorna true mientras se esta enviando la notificacion de test.
// Expuesto para tests externos.
func (s *SummaryStep) IsSending() bool {
	return s.sending
}

// Init implementa Step. Inicializa el spinner.
func (s *SummaryStep) Init(data *SetupData) tea.Cmd {
	s.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	return nil
}

// Update implementa Step.
// Maneja: spinner ticks, resultado de notificacion de test, y tecla Enter.
func (s *SummaryStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Solo procesar tick cuando estamos enviando (Pitfall W3: spinner se detiene sin ticks)
		if s.sending {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil

	case testNotificationResultMsg:
		s.sending = false
		if msg.err != nil {
			s.testErr = msg.err.Error()
		} else {
			s.testOk = true
		}
		s.done = true
		return s, tea.Quit

	case tea.KeyPressMsg:
		if msg.String() == "enter" {
			// Limpiar errores previos antes de reintentar
			s.writeErr = ""
			s.validateErr = ""

			err := writeConfig(data)
			if err != nil {
				errStr := err.Error()
				if strings.HasPrefix(errStr, "validacion:") {
					s.validateErr = errStr
				} else {
					s.writeErr = errStr
				}
				return s, nil
			}

			s.writtenPath = data.ConfigPath

			// Defensive: sin canales no podemos enviar test
			if len(data.Channels) == 0 {
				s.testErr = "sin canales configurados"
				s.done = true
				return s, tea.Quit
			}

			// Iniciar envio de test automatico (D-01: post-escritura exitosa)
			hostname, _ := os.Hostname()
			if hostname == "" {
				hostname = "VPS"
			}
			s.sending = true
			return s, tea.Batch(
				sendTestNotificationCmd(data.ValidatedBot, data.Channels[0].ChatID, hostname),
				s.spinner.Tick,
			)
		}
	}

	return s, nil
}

// View implementa Step. Renderiza el resumen completo con 5 secciones y estado final.
func (s *SummaryStep) View(data *SetupData) string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("Resumen de configuracion"))
	sb.WriteString("\n\n")

	// Seccion Telegram
	sb.WriteString("🤖 Telegram\n")
	sb.WriteString("  Token:    " + obfuscateToken(data.BotToken) + "\n")
	sb.WriteString("  Usuario:  @" + data.BotUsername + "\n")
	sb.WriteString("  Nombre:   " + data.BotDisplayName + "\n")
	sb.WriteString("\n")

	// Seccion Canales
	sb.WriteString("📢 Canales\n")
	sb.WriteString(HintStyle.Render("  Nombre               Chat ID          Prioridad") + "\n")
	for _, ch := range data.Channels {
		sb.WriteString(fmt.Sprintf("  %-20s %-16d %s\n", ch.Name, ch.ChatID, ch.Priority))
	}
	sb.WriteString("\n")

	// Seccion Servidor
	sb.WriteString("🖥️  Servidor\n")
	sb.WriteString("  Listen:  " + data.ServerListen + "\n")
	sb.WriteString("\n")

	// Seccion Base de datos
	sb.WriteString("🗃️  Base de datos\n")
	sb.WriteString("  Ruta:    " + data.DatabasePath + "\n")
	sb.WriteString("\n")

	// Seccion API Key
	sb.WriteString("🔑 API Key\n")
	if data.KeepExistingKey {
		sb.WriteString("  (mantenida)\n")
	} else {
		sb.WriteString("  " + obfuscateToken(data.GeneratedAPIKey) + "\n")
	}
	sb.WriteString("\n")

	// Estado final — tres estados nuevos de Phase 7
	if s.sending {
		// Enviando notificacion de test: mostrar config escrita + spinner
		sb.WriteString(StepDone.Render("✓ Configuracion escrita en "+s.writtenPath) + "\n\n")
		sb.WriteString(s.spinner.View() + " Enviando notificacion de test...\n")
	} else if s.testOk {
		// Test exitoso: checkmark verde + hint systemctl
		sb.WriteString(StepDone.Render("✓ Configuracion escrita en "+s.writtenPath) + "\n")
		sb.WriteString(StepDone.Render("✓ Notificacion de test enviada a Telegram") + "\n\n")
		sb.WriteString(HintStyle.Render("Para iniciar jaimito: sudo systemctl start jaimito") + "\n")
	} else if s.testErr != "" {
		// Test fallido: warning amarillo (no bloqueante — D-04)
		sb.WriteString(StepDone.Render("✓ Configuracion escrita en "+s.writtenPath) + "\n")
		sb.WriteString(WarningStyle.Render("⚠ Notificacion de test fallida: "+s.testErr) + "\n\n")
		sb.WriteString(HintStyle.Render("El config es valido. Podes iniciar: sudo systemctl start jaimito") + "\n")
	} else if s.validateErr != "" {
		sb.WriteString(ErrorStyle.Render("Error de validacion: "+s.validateErr) + "\n\n")
		sb.WriteString(HintStyle.Render("El archivo NO fue escrito. Usa Esc para corregir los valores.") + "\n")
	} else if s.writeErr != "" {
		sb.WriteString(ErrorStyle.Render("Error al escribir: "+s.writeErr) + "\n")
		if strings.Contains(strings.ToLower(s.writeErr), "permission denied") {
			sb.WriteString(HintStyle.Render("Sugerencia: ejecuta sudo jaimito setup") + "\n")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(HintStyle.Render("Presiona Enter para escribir el archivo de configuracion.") + "\n\n")
		sb.WriteString(HintStyle.Render("Enter: escribir config  │  Esc: volver  │  Ctrl+C: salir") + "\n")
	}

	return sb.String()
}

// Done implementa Step.
func (s *SummaryStep) Done() bool {
	return s.done
}

// writeConfig construye el config.Config desde SetupData, lo valida y lo escribe a disco.
func writeConfig(data *SetupData) error {
	cfg := config.Config{
		Telegram: config.TelegramConfig{Token: data.BotToken},
		Channels: data.Channels,
		Server:   config.ServerConfig{Listen: data.ServerListen},
		Database: config.DatabaseConfig{Path: data.DatabasePath},
	}

	// API keys: nueva generada o existentes mantenidas
	if !data.KeepExistingKey && data.GeneratedAPIKey != "" {
		cfg.SeedAPIKeys = []config.SeedAPIKey{
			{Name: "default", Key: data.GeneratedAPIKey},
		}
	} else if data.KeepExistingKey && data.ExistingCfg != nil {
		cfg.SeedAPIKeys = data.ExistingCfg.SeedAPIKeys
	}

	// Validar antes de escribir (CONF-04)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validacion: %w", err)
	}

	// Serializar YAML
	yamlData, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("serializar YAML: %w", err)
	}

	// Crear directorio padre si no existe
	dir := filepath.Dir(data.ConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio %s: %w", dir, err)
	}

	// Escribir con permisos 0o600
	if err := os.WriteFile(data.ConfigPath, yamlData, 0o600); err != nil {
		return fmt.Errorf("escribir config: %w", err)
	}

	return nil
}
