package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chiguire/jaimito/internal/client"
	"github.com/spf13/cobra"
)

var (
	sendChannel  string
	sendPriority string
	sendTitle    string
	sendTags     []string
	sendStdin    bool
	sendKey      string
	sendServer   string
)

var sendCmd = &cobra.Command{
	Use:   "send [body]",
	Short: "Enviar una notificacion",
	Long: `Envia una notificacion a Telegram via la API HTTP de jaimito.

Flags:
  -c, --channel    Canal destino (default: general)
  -p, --priority   Prioridad: low, normal, high (default: normal)
  -t, --title      Titulo del mensaje (aparece en negrita)
      --tags       Tags separados por coma (se muestran como #tag)
      --stdin      Leer el cuerpo del mensaje desde stdin
      --key        API key (default: variable JAIMITO_API_KEY)
      --server     Direccion del servidor (default: config o JAIMITO_SERVER)`,
	Example: `  # Mensaje simple
  jaimito send "Backup completado"

  # Con canal (-c) y prioridad (-p)
  jaimito send -c cron -p high "Backup fallo"

  # Con titulo (-t) — aparece en negrita en Telegram
  jaimito send -t "Deploy" "v1.2.3 desplegado en produccion"

  # Con tags — se muestran como #backup #cron
  jaimito send --tags backup,cron "Backup completado"

  # Desde stdin (util para pipes)
  df -h / | jaimito send --stdin -t "Disk Report" -c monitoring`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSend,
}

func init() {
	sendCmd.Flags().StringVarP(&sendChannel, "channel", "c", "", "target channel (default: general)")
	sendCmd.Flags().StringVarP(&sendPriority, "priority", "p", "", "priority: low, normal, high (default: normal)")
	sendCmd.Flags().StringVarP(&sendTitle, "title", "t", "", "message title")
	sendCmd.Flags().StringSliceVar(&sendTags, "tags", nil, "comma-separated tags")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "read body from stdin")
	sendCmd.Flags().StringVar(&sendKey, "key", "", "API key (default: JAIMITO_API_KEY env)")
	sendCmd.Flags().StringVar(&sendServer, "server", "", "server address host:port (default: from config or JAIMITO_SERVER env)")
	rootCmd.AddCommand(sendCmd)
}

func runSend(cmd *cobra.Command, args []string) error {
	// Resolve body from args or stdin.
	var body string
	switch {
	case sendStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		body = strings.TrimRight(string(data), "\n")
	case len(args) == 1:
		body = args[0]
	default:
		return fmt.Errorf("body required: provide as argument or use --stdin")
	}

	if body == "" {
		return fmt.Errorf("body must not be empty")
	}

	// Resolve authentication and server.
	apiKey, err := resolveAPIKey(sendKey)
	if err != nil {
		return err
	}
	server := resolveServer(sendServer)

	// Build request.
	req := client.NotifyRequest{
		Body:     body,
		Channel:  sendChannel,
		Priority: sendPriority,
		Tags:     sendTags,
	}
	if sendTitle != "" {
		req.Title = &sendTitle
	}

	// Send notification.
	c := client.New(server, apiKey)
	id, err := c.Notify(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	fmt.Println(id)
	return nil
}
