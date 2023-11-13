package bot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/pkg/errors"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/internals/commands"
	"my-telegram-bot/internals/logger"
)

// ResetUserCommands resets user commands after role change
func (b *MyBot) ResetUserCommands(usr *tables.TelegramUser) {
	if success, err := b.bot.SetMyCommands(
		commands.GetUserCommands(usr),
		&gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeChat{
				ChatId: usr.ID,
			},
		},
	); err != nil || !success {
		logger.Log.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Int64("user_id", usr.ID).Msg(
			"failed to reset user commands")
	}
}
