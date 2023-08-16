package commands

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/spf13/viper"
	"my-telegram-bot/internals/logger"
	"my-telegram-bot/locale"
)

type Settings struct {
	Commands []gotgbot.BotCommand
	Opts     *gotgbot.SetMyCommandsOpts
}

const DefaultKey = "general"

func getCommands(cmdKey string, includeDefaultKey bool) []gotgbot.BotCommand {
	var texts *viper.Viper
	if val, err := locale.GetCmdTranslations("en"); err != nil {
		logger.LogFatalf(err, "failed to get %s command texts", cmdKey)
	} else {
		texts = val
	}

	commands := make([]gotgbot.BotCommand, 0)

	if includeDefaultKey {
		for k, v := range texts.GetStringMapString(DefaultKey) {
			commands = append(commands, gotgbot.BotCommand{
				Command:     k,
				Description: v,
			})
		}
	}

	for k, v := range texts.GetStringMapString(cmdKey) {
		commands = append(commands, gotgbot.BotCommand{
			Command:     k,
			Description: v,
		})
	}

	return commands
}

func GetChatSpecificCommands(txtKey string, chatIDs []int64, includeDefaultKey bool) (result []Settings) {
	commands := getCommands(txtKey, includeDefaultKey)
	result = make([]Settings, len(chatIDs))
	for i, chatID := range chatIDs {
		opts := &gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeChat{
				ChatId: chatID,
			},
		}
		result[i] = Settings{
			Commands: commands,
			Opts:     opts,
		}
	}
	return
}

func GetChatMemberSpecificCommands(
	txtKey string,
	chatID int64,
	memberIDs []int64,
	includeDefaultKey bool,
) (result []Settings) {
	commands := getCommands(txtKey, includeDefaultKey)
	result = make([]Settings, len(memberIDs))
	for i, memberID := range memberIDs {
		opts := &gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeChatMember{
				ChatId: chatID,
				UserId: memberID,
			},
		}
		result[i] = Settings{
			Commands: commands,
			Opts:     opts,
		}
	}
	return
}
