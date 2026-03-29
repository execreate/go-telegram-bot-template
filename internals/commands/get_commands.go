package commands

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func getCommandsMap(txtKeys []string, lang string) map[string]string {
	var texts *viper.Viper
	if val, err := locale.GetCmdTranslations(lang); err != nil {
		logger.Log.Fatal(
			"failed to get command texts",
			zap.Error(err),
		)
	} else {
		texts = val
	}

	result := make(map[string]string)

	for i := 0; i < len(txtKeys); i++ {
		for k, v := range texts.GetStringMapString(txtKeys[i]) {
			result[k] = v
		}
	}

	return result
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

func GetUserCommands(usr *tables.TelegramUser) []gotgbot.BotCommand {
	txtKeys := []string{DefaultKey}

	if usr.IsAdmin {
		txtKeys = append(txtKeys, "admin")
	}

	return getCommandsSlice(getCommandsMap(txtKeys, usr.LanguageCode))
}
