package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chiguire/jaimito/internal/client"
	"github.com/spf13/cobra"
)

const maxOutputBytes = 3500

var (
	wrapChannel  string
	wrapPriority string
	wrapKey      string
	wrapServer   string
)

var wrapCmd = &cobra.Command{
	Use:   "wrap -- command [args...]",
	Short: "Run a command and notify on failure",
	Long: `Run a command and send a notification if it exits with a non-zero status.
The notification includes the command name, exit code, and captured output.
On success (exit code 0), wrap exits silently.`,
	Example: `  jaimito wrap -- /path/to/backup.sh
  jaimito wrap -c cron -- pg_dump -F c mydb -f /backups/mydb.dump
  jaimito wrap -c cron -p high -- /usr/local/bin/certbot renew`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runWrap,
}

func init() {
	wrapCmd.Flags().StringVarP(&wrapChannel, "channel", "c", "", "target channel (default: general)")
	wrapCmd.Flags().StringVarP(&wrapPriority, "priority", "p", "", "priority: low, normal, high (default: normal)")
	wrapCmd.Flags().StringVar(&wrapKey, "key", "", "API key (default: JAIMITO_API_KEY env)")
	wrapCmd.Flags().StringVar(&wrapServer, "server", "", "server address host:port (default: from config or JAIMITO_SERVER env)")
	rootCmd.AddCommand(wrapCmd)
}

func runWrap(cmd *cobra.Command, args []string) error {
	// Run the wrapped command and capture combined stdout+stderr.
	wrapped := exec.Command(args[0], args[1:]...)
	output, err := wrapped.CombinedOutput()

	// If the command succeeded, exit silently.
	if err == nil {
		return nil
	}

	// Extract exit code from the error.
	exitCode := 1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	// Resolve authentication and server.
	apiKey, keyErr := resolveAPIKey(wrapKey)
	if keyErr != nil {
		// Can't send notification — print error to stderr and exit with the wrapped command's code.
		fmt.Fprintf(os.Stderr, "jaimito wrap: command failed (exit %d) but cannot notify: %s\n", exitCode, keyErr)
		os.Exit(exitCode)
	}
	server := resolveServer(wrapServer)

	// Build the notification body.
	commandName := strings.Join(args, " ")
	body := formatWrapBody(commandName, exitCode, output)

	title := "Command failed"
	req := client.NotifyRequest{
		Title:    &title,
		Body:     body,
		Channel:  wrapChannel,
		Priority: wrapPriority,
	}

	// Send notification — best effort. If it fails, warn on stderr.
	c := client.New(server, apiKey)
	_, notifyErr := c.Notify(context.Background(), req)
	if notifyErr != nil {
		fmt.Fprintf(os.Stderr, "jaimito wrap: failed to send notification: %s\n", notifyErr)
	}

	// Exit with the same code as the wrapped command.
	os.Exit(exitCode)
	return nil // unreachable, but satisfies compiler
}

// formatWrapBody builds the notification body from command failure details.
// Output is truncated to maxOutputBytes to fit within Telegram's message limit.
func formatWrapBody(command string, exitCode int, output []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Command: %s\n", command)
	fmt.Fprintf(&b, "Exit code: %d\n", exitCode)

	if len(output) > 0 {
		b.WriteString("\nOutput:\n")
		out := string(output)
		if len(out) > maxOutputBytes {
			out = out[:maxOutputBytes] + "\n... (truncated)"
		}
		b.WriteString(out)
	}

	return b.String()
}
