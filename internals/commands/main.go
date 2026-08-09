package commands

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
)

type Settings struct {
	Commands []gotgbot.BotCommand
	Opts     *gotgbot.SetMyCommandsOpts
}

const DefaultKey = "general"

func GetCommands(specialUsers []*tables.TelegramUser) ([]Settings, error) {
	result := make([]Settings, 0, len(specialUsers)+1)

	generalCommands, err := getCommandsMap([]string{DefaultKey}, "en")
	if err != nil {
		return nil, err
	}
	result = append(result, Settings{
		Commands: getCommandsSlice(generalCommands),
		Opts: &gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeAllPrivateChats{},
		},
	})

	for _, usr := range specialUsers {
		userCommands, err := GetUserCommands(usr)
		if err != nil {
			return nil, err
		}
		result = append(result, Settings{
			Commands: userCommands,
			Opts: &gotgbot.SetMyCommandsOpts{
				Scope: &gotgbot.BotCommandScopeChat{
					ChatId: usr.ID,
				},
			},
		})
	}

	return result, nil
}
