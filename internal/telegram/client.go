// Package telegram provides Telegram bot validation utilities for jaimito startup.
package telegram

import (
	"context"
	"fmt"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/go-telegram/bot"
)

// ValidateToken creates a Telegram bot instance and validates the token by
// calling getMe. Returns the *bot.Bot instance for reuse in chat validation
// and later phases (dispatcher).
// Must be called before ValidateChats — getChat will fail with auth error if token is invalid.
func ValidateToken(ctx context.Context, token string) (*bot.Bot, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("invalid telegram bot token: %w", err)
	}
	return b, nil
}

// ValidateChats verifies that every unique chat_id in the channel list is reachable.
// Multiple channels may share the same chat_id (logical groupings per CONTEXT.md),
// so it deduplicates before calling getChat.
// Returns an error naming the first unreachable channel.
func ValidateChats(ctx context.Context, b *bot.Bot, channels []config.ChannelConfig) error {
	// Deduplicate chat_ids — track first channel name for each chat_id for error reporting.
	seen := make(map[int64]string)
	for _, ch := range channels {
		if _, ok := seen[ch.ChatID]; !ok {
			seen[ch.ChatID] = ch.Name
		}
	}

	for chatID, channelName := range seen {
		_, err := b.GetChat(ctx, &bot.GetChatParams{ChatID: chatID})
		if err != nil {
			return fmt.Errorf("channel %q: chat_id %d unreachable: %w", channelName, chatID, err)
		}
	}

	return nil
}
