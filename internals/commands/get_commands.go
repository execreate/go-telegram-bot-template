package commands

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/locale"
)

func getCommandsMap(txtKeys []string, lang string) (map[string]string, error) {
	texts, err := locale.GetCmdTranslations(lang)
	if err != nil {
		return nil, fmt.Errorf("failed to get command texts for locale %q: %w", lang, err)
	}

	result := make(map[string]string)

	for i := 0; i < len(txtKeys); i++ {
		for k, v := range texts.GetStringMapString(txtKeys[i]) {
			result[k] = v
		}
	}

	return result, nil
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

func GetUserCommands(usr *tables.TelegramUser) ([]gotgbot.BotCommand, error) {
	txtKeys := []string{DefaultKey}

	if usr.IsAdmin {
		txtKeys = append(txtKeys, "admin")
	}

	cmdMap, err := getCommandsMap(txtKeys, usr.LanguageCode)
	if err != nil {
		return nil, err
	}

	return getCommandsSlice(cmdMap), nil
}
