package handlers

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/pkg/errors"
	"my-telegram-bot/internals/logger"
)

// MyID replies to a command by sending user and chat ID information.
func MyID(b *gotgbot.Bot, ctx *ext.Context) error {
	var userID, chatID, linkedChatID int64
	var chatType, chatTitle string

	if ctx.EffectiveUser != nil {
		userID = ctx.EffectiveUser.Id
	}
	if ctx.EffectiveChat != nil {
		if chat, err := b.GetChat(ctx.EffectiveChat.Id, nil); err == nil {
			chatID = chat.Id
			chatType = chat.Type
			chatTitle = chat.Title
			linkedChatID = chat.LinkedChatId
		} else {
			logger.Log.Warn().Stack().Err(
				errors.Wrap(err, "wrapped error"),
			).Msg("failed to get chat info")
			chatID = ctx.EffectiveChat.Id
			chatType = ctx.EffectiveChat.Type
			chatTitle = ctx.EffectiveChat.Title
		}
	}

	txt := fmt.Sprintf(
		"User ID: `%d`\n\nChat Title: %s\nChat ID: `%d`\nChat Type: `%s`\n\nLinked Chat ID: `%d`",
		userID,
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
