package commands

import "github.com/PaulSonOfLars/gotgbot/v2"

type SpecialChatIds struct {
	Admins []int64
}

func NewSpecialChatIds() *SpecialChatIds {
	return &SpecialChatIds{
		Admins: make([]int64, 0),
	}
}

func GetCommands(ids *SpecialChatIds) (result []Settings) {
	result = make([]Settings, 0, 1+len(ids.Admins))

	result = append(result, Settings{
		Commands: getCommands(DefaultKey, false),
		Opts: &gotgbot.SetMyCommandsOpts{
			Scope: &gotgbot.BotCommandScopeAllPrivateChats{},
		},
	})

	for _, settings := range GetChatSpecificCommands(
		"admin",
		ids.Admins,
		true,
	) {
		result = append(result, settings)
	}

	return
}
