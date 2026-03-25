package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"github.com/chiguire/jaimito/internal/config"
)

// SummaryStep es la pantalla final del wizard: muestra el resumen completo de la
// configuracion, valida con config.Validate(), y escribe el YAML a disco.
type SummaryStep struct {
	confirmed   bool   // operador presiono Enter
	writeErr    string // error de escritura
	validateErr string // error de config.Validate()
	writtenPath string // path donde se escribio el config
	done        bool
}

// Init implementa Step. No hay operaciones async — todo se lee de SetupData en View().
func (s *SummaryStep) Init(data *SetupData) tea.Cmd {
	return nil
}

// Update implementa Step.
// En "enter": intenta writeConfig. Si falla, muestra error. Si ok, retorna tea.Quit.
func (s *SummaryStep) Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	if kp.String() == "enter" {
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
		s.done = true
		return s, tea.Quit
	}

	return s, nil
}

// View implementa Step. Renderiza el resumen completo con 5 secciones.
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

	// Estado final
	if s.done {
		sb.WriteString(StepDone.Render("✓ Configuracion escrita en "+s.writtenPath) + "\n\n")
		sb.WriteString(HintStyle.Render("Para iniciar jaimito: sudo systemctl start jaimito") + "\n")
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
