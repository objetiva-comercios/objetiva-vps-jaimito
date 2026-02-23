package main

import (
	"fmt"
	"os"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/spf13/cobra"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "jaimito",
	Short: "VPS push notification hub",
	Long:  "jaimito centralizes notifications from your VPS and delivers them to Telegram.",
	// Bare `jaimito` with no subcommand starts the server daemon.
	RunE:          runServe,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "/etc/jaimito/config.yaml", "path to config file")
}

// resolveServer returns the server address to connect to.
// Priority: --server flag -> JAIMITO_SERVER env -> config file -> default.
func resolveServer(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("JAIMITO_SERVER"); env != "" {
		return env
	}
	// Try loading config for server.listen (best-effort, ignore errors).
	if cfg, err := config.Load(cfgPath); err == nil && cfg.Server.Listen != "" {
		return cfg.Server.Listen
	}
	return "127.0.0.1:8080"
}

// resolveAPIKey returns the API key for authenticating with the server.
// Priority: --key flag -> JAIMITO_API_KEY env.
func resolveAPIKey(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv("JAIMITO_API_KEY"); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("API key required: set JAIMITO_API_KEY or use --key flag")
}
