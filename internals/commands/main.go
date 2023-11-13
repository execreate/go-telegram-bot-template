package commands

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"my-telegram-bot/database/tables"
)

type Settings struct {
	Commands []gotgbot.BotCommand
	Opts     *gotgbot.SetMyCommandsOpts
}

const DefaultKey = "general"

func GetCommands(specialUsers []*tables.TelegramUser) []Settings {
	result := make([]Settings, 0, len(specialUsers)+1)

	result = append(result, Settings{
		Commands: getCommandsSlice(getCommandsMap([]string{DefaultKey}, "en")),
		Opts: &gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeAllPrivateChats{},
		},
	})

	for _, usr := range specialUsers {
		result = append(result, Settings{
			Commands: GetUserCommands(usr),
			Opts: &gotgbot.SetMyCommandsOpts{
				Scope: &gotgbot.BotCommandScopeChat{
					ChatId: usr.ID,
				},
			},
		})
	}

	return result
}
