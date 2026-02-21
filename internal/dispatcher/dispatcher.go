// Package dispatcher implements the polling loop that delivers queued messages to Telegram.
package dispatcher

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/chiguire/jaimito/internal/telegram"
	gobot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	maxAttempts = 5
	baseDelay   = 2 * time.Second
)

// Start launches the dispatcher polling goroutine.
// It polls for queued messages every 1 second and delivers them sequentially.
// The goroutine stops when ctx is cancelled — all waits are context-cancellable
// for clean shutdown.
func Start(ctx context.Context, database *sql.DB, b *gobot.Bot, cfg *config.Config) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := dispatchNext(ctx, database, b, cfg); err != nil {
					slog.Error("dispatcher error", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// dispatchNext fetches the next queued message and dispatches it.
// Returns nil if no messages are queued.
func dispatchNext(ctx context.Context, database *sql.DB, b *gobot.Bot, cfg *config.Config) error {
	msg, err := db.GetNextQueued(ctx, database)
	if err != nil {
		return fmt.Errorf("get next queued: %w", err)
	}
	if msg == nil {
		return nil
	}

	return dispatch(ctx, database, b, cfg, msg)
}

// dispatch attempts to deliver a single message to Telegram, retrying up to
// maxAttempts times. 429 responses use the exact retry_after duration; other
// errors use exponential backoff. After all attempts are exhausted, the message
// is marked failed silently (logged, no further action).
func dispatch(ctx context.Context, database *sql.DB, b *gobot.Bot, cfg *config.Config, msg *db.Message) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Transition to dispatching state.
		if err := db.MarkDispatching(ctx, database, msg.ID); err != nil {
			return fmt.Errorf("mark dispatching: %w", err)
		}

		// Format message body.
		text := telegram.FormatMessage(msg)

		// Resolve the target chat_id from config.
		chatID := cfg.ChatIDForChannel(msg.Channel)
		if chatID == 0 {
			// Channel not found — should not happen if the notify handler validated.
			if dbErr := db.MarkFailed(ctx, database, msg.ID); dbErr != nil {
				slog.Error("mark failed after unknown channel", "id", msg.ID, "error", dbErr)
			}
			return fmt.Errorf("channel %q not found in config", msg.Channel)
		}

		// Send to Telegram using MarkdownV2 parse mode.
		_, sendErr := b.SendMessage(ctx, &gobot.SendMessageParams{
			ChatID:    chatID,
			Text:      text,
			ParseMode: models.ParseModeMarkdown, // "MarkdownV2" — NOT ParseModeMarkdownV1 ("Markdown")
		})

		// Record the attempt outcome regardless of success/failure.
		if recErr := db.RecordAttempt(ctx, database, msg.ID, attempt, sendErr); recErr != nil {
			slog.Error("record attempt failed", "id", msg.ID, "attempt", attempt, "error", recErr)
		}

		if sendErr == nil {
			// Delivered successfully.
			if err := db.MarkDelivered(ctx, database, msg.ID); err != nil {
				return fmt.Errorf("mark delivered: %w", err)
			}
			slog.Info("message delivered", "id", msg.ID, "channel", msg.Channel, "attempt", attempt)
			return nil
		}

		// Handle 429 Too Many Requests — respect exact retry_after.
		if gobot.IsTooManyRequestsError(sendErr) {
			tooMany := sendErr.(*gobot.TooManyRequestsError)
			waitDur := time.Duration(tooMany.RetryAfter) * time.Second
			slog.Warn("rate limited by Telegram",
				"retry_after_sec", tooMany.RetryAfter,
				"attempt", attempt,
				"id", msg.ID,
			)
			select {
			case <-time.After(waitDur):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		// Other errors: exponential backoff (2s, 4s, 8s, 16s for attempts 1-4).
		if attempt < maxAttempts {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			slog.Warn("telegram send failed, retrying",
				"attempt", attempt,
				"delay", delay,
				"error", sendErr,
				"id", msg.ID,
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// All attempts exhausted — mark failed silently per CONTEXT.md.
	if err := db.MarkFailed(ctx, database, msg.ID); err != nil {
		slog.Error("mark failed after exhausted retries", "id", msg.ID, "error", err)
	}
	slog.Error("message delivery failed after max attempts", "id", msg.ID, "attempts", maxAttempts)
	return nil
}
