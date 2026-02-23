package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/spf13/cobra"
)

var keyName string

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long:  "Create, list, and revoke API keys for authenticating with the jaimito HTTP API.",
}

var keysCreateCmd = &cobra.Command{
	Use:     "create",
	Short:   "Create a new API key",
	Example: "  jaimito keys create --name backup-service",
	RunE:    runKeysCreate,
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active API keys",
	RunE:  runKeysList,
}

var keysRevokeCmd = &cobra.Command{
	Use:     "revoke [id]",
	Short:   "Revoke an API key",
	Example: "  jaimito keys revoke 550e8400-e29b-41d4-a716-446655440000",
	Args:    cobra.ExactArgs(1),
	RunE:    runKeysRevoke,
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "name for the new API key (required)")
	keysCreateCmd.MarkFlagRequired("name")

	keysCmd.AddCommand(keysCreateCmd, keysListCmd, keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}

// openDB loads config and opens the database with schema applied.
// Used by keys subcommands that need direct DB access.
func openDB() (*sql.DB, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.ApplySchema(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}
	return database, nil
}

func runKeysCreate(cmd *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	key, err := db.CreateKey(context.Background(), database, keyName)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}

	fmt.Println(key)
	return nil
}

func runKeysList(cmd *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	keys, err := db.ListKeys(context.Background(), database)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println("No API keys found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tCREATED\tLAST USED")
	for _, k := range keys {
		lastUsed := "-"
		if k.LastUsedAt != nil {
			lastUsed = *k.LastUsedAt
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k.ID, k.Name, k.CreatedAt, lastUsed)
	}
	w.Flush()
	return nil
}

func runKeysRevoke(cmd *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.RevokeKey(context.Background(), database, args[0]); err != nil {
		return err
	}

	fmt.Printf("Key %s revoked.\n", args[0])
	return nil
}
