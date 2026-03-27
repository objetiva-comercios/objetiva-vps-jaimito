package telegram

import (
	"strings"

	"github.com/chiguire/jaimito/internal/db"
	"github.com/go-telegram/bot"
)

// priorityEmoji maps priority strings to traffic light emoji per CONTEXT.md.
var priorityEmoji = map[string]string{
	"low":      "🟢",
	"normal":   "🟡",
	"high":     "🔴",
	"critical": "🔴",
}

// FormatMessage produces a MarkdownV2-formatted string for the given message.
//
// Layout (per CONTEXT.md locked decisions):
//   - With title: emoji + bold title on one line, escaped body below
//   - Without title: emoji + escaped body on one line
//   - Tags rendered as hashtags on a new line after body: #disk #backup
//   - All user text is escaped via bot.EscapeMarkdown
func FormatMessage(msg *db.Message) string {
	var sb strings.Builder

	emoji, ok := priorityEmoji[msg.Priority]
	if !ok {
		emoji = "🟡"
	}

	if msg.Title != nil && *msg.Title != "" {
		// emoji + bold title on first line, body below
		sb.WriteString(emoji)
		sb.WriteString(" *")
		sb.WriteString(bot.EscapeMarkdown(*msg.Title))
		sb.WriteString("*\n")
		sb.WriteString(bot.EscapeMarkdown(msg.Body))
	} else {
		// emoji + body on one line
		sb.WriteString(emoji)
		sb.WriteString(" ")
		sb.WriteString(bot.EscapeMarkdown(msg.Body))
	}

	// Tags as hashtags on a new line.
	// '#' MUST be escaped as '\#' in MarkdownV2 — Telegram rejects unescaped '#'.
	// The rendered message still shows '#tag' correctly in the chat.
	if len(msg.Tags) > 0 {
		sb.WriteString("\n")
		for i, tag := range msg.Tags {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString("\\#")
			sb.WriteString(bot.EscapeMarkdown(tag))
		}
	}

	return sb.String()
}
