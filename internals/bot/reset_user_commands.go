package bot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/commands"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"go.uber.org/zap"
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
		logger.Log.Error(
			"failed to reset user commands",
			zap.Error(err),
			zap.Int64("user_id", usr.ID),
		)
	}
}
