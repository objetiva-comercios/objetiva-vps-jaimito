package setup

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/go-telegram/bot"
	"golang.org/x/term"

	"github.com/chiguire/jaimito/internal/config"
)

// SetupData contiene todos los valores recopilados por el wizard.
// Se pasa por puntero a cada step — mutacion directa, sin message-passing.
type SetupData struct {
	ConfigPath   string
	ExistingCfg  *config.Config // nil si no existe
	ConfigErr    error          // error de carga si config es invalido
	ConfigExists bool           // true si el archivo de config existe en disco
	Mode         string         // "new", "edit", "fresh"

	// Campos del bot de Telegram (populados por BotTokenStep)
	BotToken       string              // token validado (plain text)
	BotUsername    string              // @username del bot (sin @)
	BotDisplayName string              // FirstName del bot
	ValidatedBot   *bot.Bot            // instancia reutilizable para GetChat en steps siguientes
	Channels       []config.ChannelConfig // canales acumulados (general + extras)
}

// Step es la interfaz que implementa cada pantalla del wizard.
// NO implementa tea.Model — el wizard principal es el unico tea.Model.
type Step interface {
	Init(data *SetupData) tea.Cmd
	Update(msg tea.Msg, data *SetupData) (Step, tea.Cmd)
	View(data *SetupData) string
	Done() bool
}

// stepNames son los nombres visibles de los 7 steps en el sidebar.
var stepNames = []string{
	"Bienvenida",
	"Bot Token",
	"Canal General",
	"Canales Extra",
	"Servidor",
	"Base de Datos",
	"Resumen",
}

// WizardModel es el unico tea.Model del wizard.
type WizardModel struct {
	steps         []Step
	currentStep   int
	sidebarOffset int // steps antes del primer step visible en la sidebar
	data          *SetupData
	quitting      bool
	confirmExit   bool
	completedMap  map[int]bool
}

// NewWizardModel construye un WizardModel listo para usar.
// Infiere configExists desde existingCfg y configErr.
func NewWizardModel(cfgPath string, existingCfg *config.Config, configErr error) WizardModel {
	configExists := existingCfg != nil || configErr != nil
	return NewWizardModelWithExists(cfgPath, existingCfg, configErr, configExists)
}

// NewWizardModelWithExists construye un WizardModel con informacion explicita sobre
// si el archivo de config existe en disco.
func NewWizardModelWithExists(cfgPath string, existingCfg *config.Config, configErr error, configExists bool) WizardModel {
	data := &SetupData{
		ConfigPath:   cfgPath,
		ExistingCfg:  existingCfg,
		ConfigErr:    configErr,
		ConfigExists: configExists,
	}

	// Steps visibles en la sidebar (los 7 del wizard)
	visibleSteps := []Step{
		&WelcomeStep{},
		&BotTokenStep{},
		&PlaceholderStep{name: "Canal General"},
		&PlaceholderStep{name: "Canales Extra"},
		&PlaceholderStep{name: "Servidor"},
		&PlaceholderStep{name: "Base de Datos"},
		&PlaceholderStep{name: "Resumen"},
	}

	// Si hay config (valido o invalido), insertar DetectConfigStep como step 0 (interno).
	// Si no hay config, saltar directamente al wizard sin mostrar DetectConfigStep.
	var steps []Step
	sidebarOffset := 0

	if configExists {
		steps = append([]Step{&DetectConfigStep{}}, visibleSteps...)
		sidebarOffset = 1
	} else {
		// No hay config: setear Mode="new" directamente
		data.Mode = "new"
		steps = visibleSteps
	}

	return WizardModel{
		steps:         steps,
		currentStep:   0,
		sidebarOffset: sidebarOffset,
		data:          data,
		completedMap:  make(map[int]bool),
	}
}

// Init implementa tea.Model. Llama Init del primer step.
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) > 0 {
		return m.steps[0].Init(m.data)
	}
	return nil
}

// Update implementa tea.Model.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// CRITICO: usar tea.KeyPressMsg (v2), NO tea.KeyMsg (v1) — ver Pitfall W3
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.confirmExit {
				m.quitting = true
				return m, tea.Quit
			}
			m.confirmExit = true
			return m, nil
		case "esc", "up":
			// Reset confirmExit si estaba activo
			if m.confirmExit {
				m.confirmExit = false
				return m, nil
			}
			if m.currentStep > 0 {
				m.currentStep--
			}
			return m, nil
		default:
			// Cualquier otra tecla cancela confirmExit
			if m.confirmExit {
				m.confirmExit = false
			}
		}
	}

	// Delegar al step activo
	newStep, cmd := m.steps[m.currentStep].Update(msg, m.data)
	m.steps[m.currentStep] = newStep

	if newStep.Done() && m.currentStep < len(m.steps)-1 {
		m.completedMap[m.currentStep] = true
		m.currentStep++
		initCmd := m.steps[m.currentStep].Init(m.data)
		return m, tea.Batch(cmd, initCmd)
	}

	return m, cmd
}

// View implementa tea.Model.
func (m WizardModel) View() tea.View {
	var content string

	if m.quitting {
		content = "Saliendo...\n"
	} else if m.confirmExit {
		content = ErrorStyle.Render("Seguro que queres salir? Se pierden los datos ingresados. (Ctrl+C para confirmar, cualquier otra tecla para cancelar)\n")
	} else {
		content = renderLayout(m)
	}

	return tea.NewView(content)
}

// renderSidebar genera la barra lateral con los steps y su estado.
// sidebarStep es el indice del step activo dentro de stepNames (ya ajustado por offset).
// completedSteps es el mapa de steps completados (indices en el slice de steps del model).
// sidebarOffset es la cantidad de steps internos antes del primer step visible.
func renderSidebar(sidebarStep int, completedSteps map[int]bool, sidebarOffset int) string {
	var sb strings.Builder

	for i, name := range stepNames {
		// Convertir indice de sidebar a indice en steps slice
		stepsIdx := i + sidebarOffset
		var line string
		if i == sidebarStep {
			// El step activo siempre se muestra como activo, incluso si fue completado antes
			line = StepActive.Render("▸ " + name)
		} else if completedSteps[stepsIdx] {
			line = StepDone.Render("✓ " + name)
		} else {
			line = StepPending.Render("  " + name)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Contador [N/7]
	counter := fmt.Sprintf("[%d/7]", sidebarStep+1)
	sb.WriteString(HintStyle.Render(counter))
	sb.WriteString("\n")

	return sb.String()
}

// renderLayout compone el layout completo: sidebar + separador + contenido del step.
func renderLayout(m WizardModel) string {
	// Calcular el indice de sidebar: currentStep menos los steps internos antes de la sidebar
	sidebarStep := m.currentStep - m.sidebarOffset
	if sidebarStep < 0 {
		sidebarStep = 0
	}
	sidebar := renderSidebar(sidebarStep, m.completedMap, m.sidebarOffset)

	// Contenido del step actual
	stepContent := m.steps[m.currentStep].View(m.data)

	// Separador vertical
	separator := lipgloss.NewStyle().Foreground(ColorGray).Render("│")

	// Unir sidebar + separador + contenido
	sidebarLines := strings.Split(sidebar, "\n")
	contentLines := strings.Split(stepContent, "\n")

	// Rellenar el sidebar hasta la altura del contenido si es necesario
	maxLines := len(sidebarLines)
	if len(contentLines) > maxLines {
		maxLines = len(contentLines)
	}
	for len(sidebarLines) < maxLines {
		sidebarLines = append(sidebarLines, "")
	}
	for len(contentLines) < maxLines {
		contentLines = append(contentLines, "")
	}

	var layout strings.Builder
	for i := 0; i < maxLines; i++ {
		// Padding del sidebar a ancho fijo
		sbLine := fmt.Sprintf("%-20s", sidebarLines[i])
		layout.WriteString(sbLine)
		layout.WriteString(" ")
		layout.WriteString(separator)
		layout.WriteString("  ")
		if i < len(contentLines) {
			layout.WriteString(contentLines[i])
		}
		layout.WriteString("\n")
	}

	// Barra de atajos inferior
	hints := HintStyle.Render("Enter: continuar  │  Esc: volver  │  Ctrl+C: salir")
	layout.WriteString("\n")
	layout.WriteString(hints)
	layout.WriteString("\n")

	return layout.String()
}

// FormatNonInteractiveError retorna el mensaje de error formateado cuando
// jaimito setup se ejecuta en una terminal no-interactiva.
func FormatNonInteractiveError() string {
	msg := "Error: jaimito setup requiere una terminal interactiva.\n\n" +
		"Si estas usando curl | bash, el instalador ya redirige stdin automaticamente.\n" +
		"Para ejecutar manualmente: jaimito setup --config /etc/jaimito/config.yaml"
	return ErrorStyle.Render(msg)
}

// RunWizard lanza el wizard bubbletea. Infiere configExists desde existingCfg y configErr.
// Mantener compatibilidad con tests existentes.
func RunWizard(cfgPath string, existingCfg *config.Config, configErr error) error {
	return RunWizardWithExists(cfgPath, existingCfg, configErr, existingCfg != nil || configErr != nil)
}

// RunWizardWithExists lanza el wizard bubbletea con informacion explicita sobre
// si el archivo de config existe en disco.
func RunWizardWithExists(cfgPath string, existingCfg *config.Config, configErr error, configExists bool) error {
	model := NewWizardModelWithExists(cfgPath, existingCfg, configErr, configExists)

	var opts []tea.ProgramOption

	// Pitfall W2: si stdout no es TTY, abrir /dev/tty para output
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err == nil {
			defer tty.Close()
			opts = append(opts, tea.WithOutput(tty))
		}
	}

	p := tea.NewProgram(model, opts...)
	_, err := p.Run()
	return err
}
