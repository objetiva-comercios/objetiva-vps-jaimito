package main

import (
	"fmt"
	"os"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupCmd = &cobra.Command{
	Use:          "setup",
	Short:        "Configurar jaimito de forma interactiva",
	SilenceUsage: true,
	RunE:         runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	// CRITICO: verificar terminal ANTES de tea.NewProgram (Pitfall W1)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		errMsg := setup.FormatNonInteractiveError()
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(1)
	}

	// Detectar config existente — guardar tanto cfg como err
	cfg, configErr := config.Load(cfgPath)

	return setup.RunWizard(cfgPath, cfg, configErr)
}
