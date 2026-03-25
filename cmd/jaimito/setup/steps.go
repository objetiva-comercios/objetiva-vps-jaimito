package setup

import (
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
