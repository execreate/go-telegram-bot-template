package handlers

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/internals/logger"
)

// MyID replies to a command by sending user and chat ID information.
func MyID(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveUser == nil {
		return nil
	}
	user := ctx.Data["db_user"].(*tables.TelegramUser)
	if !user.IsAdmin {
		return nil
	}

	var chatID, linkedChatID int64
	var chatType, chatTitle string

	if ctx.EffectiveChat != nil {
		if chat, err := b.GetChat(ctx.EffectiveChat.Id, nil); err == nil {
			chatID = chat.Id
			chatType = chat.Type
			chatTitle = chat.Title
			linkedChatID = chat.LinkedChatId
		} else {
			logger.LogWarningf("failed to get chat info: %v", err)
			chatID = ctx.EffectiveChat.Id
			chatType = ctx.EffectiveChat.Type
			chatTitle = ctx.EffectiveChat.Title
		}
	}

	txt := fmt.Sprintf(
		"User ID: `%d`\n\nChat Title: %s\nChat ID: `%d`\nChat Type: `%s`\n\nLinked Chat ID: `%d`",
		user.ID,
		chatTitle,
		chatID,
		chatType,
		linkedChatID,
	)

	_, err := ctx.EffectiveMessage.Reply(
		b,
		txt,
		&gotgbot.SendMessageOpts{
			ParseMode: gotgbot.ParseModeMarkdownV2,
		},
	)
	return err
}
