package main

import (
	"context"
	"fmt"

	"github.com/chiguire/jaimito/internal/client"
	"github.com/spf13/cobra"
)

var (
	metricName   string
	metricValue  float64
	metricKey    string
	metricServer string
)

var metricCmd = &cobra.Command{
	Use:   "metric",
	Short: "Enviar una metrica manual",
	Long: `Envia una lectura de metrica al servidor via POST /api/v1/metrics.

La metrica debe estar definida en config.yaml y existir en la base de datos.
Requiere API key para autenticacion.`,
	Example: `  # Enviar una lectura manual
  jaimito metric -n disk_root --value 85.5

  # Con server y key explicitos
  jaimito metric -n custom_metric --value 42 --server 127.0.0.1:8080 --key sk-xxx`,
	RunE: runMetric,
}

func init() {
	metricCmd.Flags().StringVarP(&metricName, "name", "n", "", "metric name (required)")
	metricCmd.Flags().Float64Var(&metricValue, "value", 0, "metric value (required)")
	metricCmd.Flags().StringVar(&metricKey, "key", "", "API key (default: JAIMITO_API_KEY env)")
	metricCmd.Flags().StringVar(&metricServer, "server", "", "server address host:port")
	metricCmd.MarkFlagRequired("name")
	metricCmd.MarkFlagRequired("value")
	rootCmd.AddCommand(metricCmd)
}

func runMetric(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveAPIKey(metricKey)
	if err != nil {
		return err
	}
	server := resolveServer(metricServer)

	c := client.New(server, apiKey)
	resp, err := c.PostMetric(context.Background(), client.PostMetricRequest{
		Name:  metricName,
		Value: metricValue,
	})
	if err != nil {
		return fmt.Errorf("failed to send metric: %w", err)
	}

	fmt.Printf("%s = %.2f (recorded at %s)\n", resp.Name, resp.Value, resp.RecordedAt)
	return nil
}
