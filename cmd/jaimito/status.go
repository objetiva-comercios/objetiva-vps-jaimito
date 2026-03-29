package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/chiguire/jaimito/internal/client"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var statusServer string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostrar estado de las metricas",
	Long:  "Consulta GET /api/v1/metrics y muestra las metricas actuales en formato tabla.",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusServer, "server", "", "server address host:port")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	server := resolveServer(statusServer)
	c := client.New(server, "")

	metrics, err := c.GetMetrics(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "dial") {
			return fmt.Errorf("server not reachable at %s — is jaimito running?", server)
		}
		return err
	}

	if len(metrics) == 0 {
		fmt.Println("No metrics configured.")
		fmt.Println("")
		fmt.Println("Para activar metricas, agregar la seccion 'metrics' en config.yaml:")
		fmt.Println("  sudo nano /etc/jaimito/config.yaml")
		fmt.Println("")
		fmt.Println("Ejemplo minimo:")
		fmt.Println("  metrics:")
		fmt.Println("    retention: \"7d\"")
		fmt.Println("    alert_cooldown: \"30m\"")
		fmt.Println("    collect_interval: \"60s\"")
		fmt.Println("    definitions:")
		fmt.Println("      - name: disk_root")
		fmt.Println("        command: \"df / | awk 'NR==2 {print $5}' | tr -d '%'\"")
		fmt.Println("        category: system")
		fmt.Println("        type: gauge")
		fmt.Println("        thresholds:")
		fmt.Println("          warning: 80")
		fmt.Println("          critical: 90")
		fmt.Println("")
		fmt.Println("Ver mas ejemplos en: configs/config.example.yaml del repositorio.")
		fmt.Println("Reiniciar despues de cambiar config: sudo systemctl restart jaimito")
		return nil
	}

	renderMetricsTable(metrics)
	return nil
}

func renderMetricsTable(metrics []client.MetricRow) {
	colorEnabled := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	statusIcons := map[string]string{
		"ok":       "✅",
		"warning":  "⚠️ ",
		"critical": "🔴",
		"":         "—",
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVALUE\tSTATUS\tUPDATED")
	fmt.Fprintln(w, "----\t-----\t------\t-------")

	for _, m := range metrics {
		valueStr := "—"
		if m.LastValue != nil {
			valueStr = fmt.Sprintf("%.2f", *m.LastValue)
		}

		icon := statusIcons[m.LastStatus]
		if icon == "" {
			icon = "—"
		}
		statusStr := icon + " " + m.LastStatus

		if colorEnabled {
			switch m.LastStatus {
			case "warning":
				statusStr = "\033[33m" + statusStr + "\033[0m"
			case "critical":
				statusStr = "\033[31m" + statusStr + "\033[0m"
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, valueStr, statusStr, m.UpdatedAt)
	}

	w.Flush()
}
