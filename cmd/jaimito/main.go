package main

import (
	"context"
	"errors"
	"flag"
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
)

func main() {
	// 1. Initialize logging: structured text output to stdout for journald capture.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// 2. Parse config path flag — defaults to /etc/jaimito/config.yaml for production.
	var configPath string
	flag.StringVar(&configPath, "config", "/etc/jaimito/config.yaml", "path to config file")
	flag.Parse()

	// 3. Load and validate config — fail fast on missing or malformed config.
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 4. Set up signal context — SIGTERM and SIGINT trigger graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// 5. Validate Telegram token — calls getMe to verify the token is accepted.
	tgBot, err := telegram.ValidateToken(ctx, cfg.Telegram.Token)
	if err != nil {
		slog.Error("telegram token validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("telegram bot validated")

	// 6. Validate Telegram chat IDs — calls getChat for each unique configured chat.
	if err := telegram.ValidateChats(ctx, tgBot, cfg.Channels); err != nil {
		slog.Error("telegram chat validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("telegram chats validated", "count", len(cfg.Channels))

	// 7. Open database — auto-creates the file and parent directory if absent.
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// 8. Apply schema migrations — idempotent, safe to run on every startup.
	if err := db.ApplySchema(database); err != nil {
		slog.Error("failed to apply database schema", "error", err)
		os.Exit(1)
	}
	slog.Info("database schema applied")

	// 9. Reclaim stuck messages — resets dispatching -> queued for crash recovery.
	reclaimed, err := db.ReclaimStuck(ctx, database)
	if err != nil {
		slog.Error("failed to reclaim stuck messages", "error", err)
		os.Exit(1)
	}
	if reclaimed > 0 {
		slog.Info("reclaimed stuck messages", "count", reclaimed)
	}

	// 10. Seed API keys — idempotent bootstrap from config seed_api_keys.
	if len(cfg.SeedAPIKeys) > 0 {
		if err := db.SeedKeys(ctx, database, cfg.SeedAPIKeys); err != nil {
			slog.Error("failed to seed API keys", "error", err)
			os.Exit(1)
		}
		slog.Info("API keys seeded", "count", len(cfg.SeedAPIKeys))
	}

	// 11. Start dispatcher — polls for queued messages every 1s, delivers via Telegram.
	dispatcher.Start(ctx, database, tgBot, cfg)

	// 12. Start cleanup scheduler — purges old messages in background goroutine.
	cleanup.Start(ctx, database, 24*time.Hour)

	// 13. Start HTTP server — serves API endpoints for notification ingestion.
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

	// 14. Log ready state — all components initialized successfully.
	slog.Info("jaimito started",
		"addr", cfg.Server.Listen,
		"channels", len(cfg.Channels),
		"db", cfg.Database.Path,
	)

	// 15. Wait for shutdown signal — graceful shutdown with 30s timeout.
	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}
}
