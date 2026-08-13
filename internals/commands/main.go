package commands

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/locale"
)

type Settings struct {
	Commands []gotgbot.BotCommand
	Opts     *gotgbot.SetMyCommandsOpts
}

type commandScope int

const (
	// these are default telegram scopes:
	commandScopeDefault commandScope = iota
	commandScopeAllPrivateChats
	commandScopeAllGroupChats
	commandScopeAllChatAdministrators

	// and these are special chat-specific scopes:
	commandScopeChatAdministrators
	commandScopeChatMember

	commandScopeBotAdmin
	commandScopeBotOwner
)

var commandScopeKeys = map[commandScope]string{
	commandScopeDefault:               "default",
	commandScopeAllPrivateChats:       "all_private_chats",
	commandScopeAllGroupChats:         "all_group_chats",
	commandScopeAllChatAdministrators: "all_chat_administrators",

	commandScopeChatAdministrators: "chat_administrators",
	commandScopeChatMember:         "chat_member",

	commandScopeBotAdmin: "bot_admin",
	commandScopeBotOwner: "bot_owner",
}

func getCommandSettings(
	commands map[string]string,
	scope gotgbot.BotCommandScope,
	language string,
) Settings {
	if len(commands) == 0 {
		return Settings{
			Commands: make([]gotgbot.BotCommand, 0),
			Opts: &gotgbot.SetMyCommandsOpts{
				LanguageCode: language,
				Scope:        scope,
			},
		}
	}

	return Settings{
		Commands: getCommandsSlice(commands),
		Opts: &gotgbot.SetMyCommandsOpts{
			LanguageCode: language,
			Scope:        scope,
		},
	}
}

// GetCommands returns a slice of Settings for all commands
func GetCommands(
	botOwnersOrAdmins []*tables.TelegramUser,
	language string,
) ([]Settings, error) {
	result := make([]Settings, 0, len(botOwnersOrAdmins)+4)

	commands, err := locale.GetCmdTranslations(language)
	if err != nil {
		return nil, err
	}

	// BotCommandScopeDefault
	result = append(result, getCommandSettings(
		commands.GetStringMapString(commandScopeKeys[commandScopeDefault]),
		&gotgbot.BotCommandScopeDefault{},
		language,
	))

	// BotCommandScopeAllPrivateChats
	result = append(result, getCommandSettings(
		commands.GetStringMapString(commandScopeKeys[commandScopeAllPrivateChats]),
		&gotgbot.BotCommandScopeAllPrivateChats{},
		language,
	))

	// BotCommandScopeAllGroupChats
	result = append(result, getCommandSettings(
		commands.GetStringMapString(commandScopeKeys[commandScopeAllGroupChats]),
		&gotgbot.BotCommandScopeAllGroupChats{},
		language,
	))

	// BotCommandScopeAllChatAdministrators
	result = append(result, getCommandSettings(
		commands.GetStringMapString(commandScopeKeys[commandScopeAllChatAdministrators]),
		&gotgbot.BotCommandScopeAllChatAdministrators{},
		language,
	))

	// BotCommandScopeChatAdministrators: needs a specific chatID, uncomment later if needed
	//result = append(result, getCommandSettings(
	//	commands.GetStringMapString(commandScopeKeys[commandScopeChatAdministrators]),
	//	&gotgbot.BotCommandScopeChatAdministrators{
	//		ChatId: 0, // todo: don't for get to adjust; also don't forget to adjust result slice length above
	//	},
	//	language,
	//))

	// BotCommandScopeChatMember: needs a specific chatID, uncomment later if needed
	//result = append(result, getCommandSettings(
	//	commands.GetStringMapString(commandScopeKeys[commandScopeChatMember]),
	//	&gotgbot.BotCommandScopeChatMember{
	//		ChatId: 0, // todo: don't for get to adjust; also don't forget to adjust result slice length above
	//	},
	//	language,
	//))

	botAdminCommands := commands.GetStringMapString(commandScopeKeys[commandScopeBotAdmin])
	botOwnerCommands := commands.GetStringMapString(commandScopeKeys[commandScopeBotOwner])

	for _, usr := range botOwnersOrAdmins {
		if usr.IsAdmin {
			result = append(result, getCommandSettings(
				botAdminCommands,
				&gotgbot.BotCommandScopeChat{
					ChatId: usr.ID,
				},
				language,
			))
		}
		if usr.IsOwner {
			result = append(result, getCommandSettings(
				botOwnerCommands,
				&gotgbot.BotCommandScopeChat{
					ChatId: usr.ID,
				},
				language,
			))
		}
	}

	return result, nil
}
