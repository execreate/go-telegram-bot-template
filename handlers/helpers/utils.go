package helpers

import (
	"fmt"
	"strings"

	"github.com/execreate/go-telegram-bot-template/database/tables"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func ContainsMessageViaBot(msg, botUsername string, ctx *ext.Context) bool {
	return ctx.EffectiveMessage != nil &&
		ctx.EffectiveMessage.Text == msg &&
		ctx.EffectiveMessage.ForwardOrigin.GetDate() == 0 &&
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

func EscapeMarkdownChars(text string) string {
	replacer := strings.NewReplacer(
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
	return replacer.Replace(text)
}
