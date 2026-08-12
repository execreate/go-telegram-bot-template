package commands

import (
	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/locale"
)

var userCommandsNotFound = errors.New("commands for this user not found")

func GetUserCommands(usr *tables.TelegramUser) ([]gotgbot.BotCommand, error) {
	commands, err := locale.GetCmdTranslations(usr.LanguageCode)
	if err != nil {
		return nil, err
	}

	var usrCommands map[string]string

	if usr.IsAdmin {
		usrCommands = commands.GetStringMapString(commandScopeKeys[commandScopeBotAdmin])
	} else if usr.IsOwner {
		usrCommands = commands.GetStringMapString(commandScopeKeys[commandScopeBotOwner])
	} else {
		return nil, userCommandsNotFound
	}

	return getCommandsSlice(usrCommands), nil
}

func getCommandsSlice(cmdMap map[string]string) []gotgbot.BotCommand {
	result := make([]gotgbot.BotCommand, 0, len(cmdMap))

	for k, v := range cmdMap {
		result = append(result, gotgbot.BotCommand{
			Command:     k,
			Description: v,
		})
	}

	return result
}
