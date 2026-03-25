// Package setup contiene el wizard interactivo de configuracion de jaimito.
package setup

import lipgloss "charm.land/lipgloss/v2"

// Paleta de colores del wizard — locked decision (CONTEXT.md).
var (
	ColorCyan   = lipgloss.Color("#00BFFF")
	ColorGreen  = lipgloss.Color("#00FF87")
	ColorRed    = lipgloss.Color("#FF5F56")
	ColorYellow = lipgloss.Color("#FFBD2E")
	ColorGray   = lipgloss.Color("#666666")
)

// Estilos para el sidebar de progreso.
var (
	StepActive  = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	StepDone    = lipgloss.NewStyle().Foreground(ColorGreen)
	StepPending = lipgloss.NewStyle().Foreground(ColorGray)
)

// Estilos generales del wizard.
var (
	TitleStyle = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	ErrorStyle = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
	HintStyle  = lipgloss.NewStyle().Foreground(ColorGray)
)
