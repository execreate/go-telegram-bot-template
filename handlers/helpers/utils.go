package helpers

import (
	"fmt"
	"strings"

	"github.com/execreate/go-telegram-bot-template/database/tables"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// ContainsMessageViaBot reports whether ctx holds an original (not forwarded) message
// with the exact text msg, sent inline via the bot botUsername.
func ContainsMessageViaBot(msg, botUsername string, ctx *ext.Context) bool {
	// ForwardOrigin is an interface and is nil unless the message was forwarded, so it
	// must be compared rather than called through.
	return ctx.EffectiveMessage != nil &&
		ctx.EffectiveMessage.Text == msg &&
		ctx.EffectiveMessage.ForwardOrigin == nil &&
		ctx.EffectiveMessage.ViaBot != nil &&
		ctx.EffectiveMessage.ViaBot.Username == botUsername
}

func FormDataHasKeys(keys []string, formData map[string][]string) bool {
	if formData == nil {
		return false
	}

	for k := range keys {
		if val, ok := formData[keys[k]]; !ok || len(val) == 0 {
			return false
		}
	}

	return true
}

func GetUserMention(user *tables.TelegramUser) string {
	if user == nil {
		return ""
	}

	if !user.Username.Valid {
		return fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", user.ID, user.FullName())
	}

	return fmt.Sprintf("@%s", user.Username.String)
}

// escapeMarkdownV2 escapes every character Telegram treats as special in MarkdownV2.
// The backslash pair must come first: strings.NewReplacer matches in argument order,
// so any later pair would otherwise have its own backslash escaped again.
var escapeMarkdownV2 = strings.NewReplacer(
	"\\", "\\\\",
	"_", "\\_",
	"*", "\\*",
	"[", "\\[",
	"]", "\\]",
	"(", "\\(",
	")", "\\)",
	"~", "\\~",
	"`", "\\`",
	">", "\\>",
	"#", "\\#",
	"+", "\\+",
	"-", "\\-",
	"=", "\\=",
	"|", "\\|",
	"{", "\\{",
	"}", "\\}",
	".", "\\.",
	"!", "\\!",
)

// EscapeMarkdownChars escapes text for safe interpolation into a MarkdownV2 message.
func EscapeMarkdownChars(text string) string {
	return escapeMarkdownV2.Replace(text)
}
