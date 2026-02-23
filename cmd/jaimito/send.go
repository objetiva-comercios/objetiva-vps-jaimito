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
	Short: "Send a notification",
	Long:  "Send a notification to Telegram via the jaimito server HTTP API.",
	Example: `  jaimito send "Backup completed successfully"
  jaimito send -c cron -p high "Backup failed"
  jaimito send -t "Deploy" "v1.2.3 deployed to production"
  echo "disk usage: 90%" | jaimito send --stdin -c monitoring`,
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
