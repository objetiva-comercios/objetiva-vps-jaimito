package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chiguire/jaimito/internal/api"
	"github.com/chiguire/jaimito/internal/cleanup"
	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/chiguire/jaimito/internal/dispatcher"
	"github.com/chiguire/jaimito/internal/telegram"
	"github.com/spf13/cobra"
)

func runServe(cmd *cobra.Command, args []string) error {
	// 1. Initialize logging: structured text output to stdout for journald capture.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// 2. Load and validate config — fail fast on missing or malformed config.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 3. Set up signal context — SIGTERM and SIGINT trigger graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// 4. Validate Telegram token — calls getMe to verify the token is accepted.
	tgBot, err := telegram.ValidateToken(ctx, cfg.Telegram.Token)
	if err != nil {
		return fmt.Errorf("telegram token validation failed: %w", err)
	}
	slog.Info("telegram bot validated")

	// 5. Validate Telegram chat IDs — calls getChat for each unique configured chat.
	if err := telegram.ValidateChats(ctx, tgBot, cfg.Channels); err != nil {
		return fmt.Errorf("telegram chat validation failed: %w", err)
	}
	slog.Info("telegram chats validated", "count", len(cfg.Channels))

	// 6. Open database — auto-creates the file and parent directory if absent.
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// 7. Apply schema migrations — idempotent, safe to run on every startup.
	if err := db.ApplySchema(database); err != nil {
		return fmt.Errorf("failed to apply database schema: %w", err)
	}
	slog.Info("database schema applied")

	// 8. Reclaim stuck messages — resets dispatching -> queued for crash recovery.
	reclaimed, err := db.ReclaimStuck(ctx, database)
	if err != nil {
		return fmt.Errorf("failed to reclaim stuck messages: %w", err)
	}
	if reclaimed > 0 {
		slog.Info("reclaimed stuck messages", "count", reclaimed)
	}

	// 9. Seed API keys — idempotent bootstrap from config seed_api_keys.
	if len(cfg.SeedAPIKeys) > 0 {
		if err := db.SeedKeys(ctx, database, cfg.SeedAPIKeys); err != nil {
			return fmt.Errorf("failed to seed API keys: %w", err)
		}
		slog.Info("API keys seeded", "count", len(cfg.SeedAPIKeys))
	}

	// 10. Start dispatcher — polls for queued messages every 1s, delivers via Telegram.
	dispatcher.Start(ctx, database, tgBot, cfg)

	// 11. Start cleanup scheduler — purges old messages in background goroutine.
	cleanup.Start(ctx, database, 24*time.Hour)

	// 12. Start HTTP server — serves API endpoints for notification ingestion.
	router := api.NewRouter(database, cfg)
	server := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 13. Log ready state — all components initialized successfully.
	slog.Info("jaimito started",
		"addr", cfg.Server.Listen,
		"channels", len(cfg.Channels),
		"db", cfg.Database.Path,
	)

	// 14. Wait for shutdown signal — graceful shutdown with 30s timeout.
	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	return nil
}
