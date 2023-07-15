package main

import (
	tgbotHandlers "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"my-telegram-bot/handlers"
	"my-telegram-bot/handlers/context"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/static"
)

func main() {
	config := configure()
	myBot := bot.NewBot(config)

	// enrich context data
	myBot.AddHandlerToGroup(context.NewUserContextHandler(myBot.UsersCache), -1)
	myBot.AddHandlerToGroup(context.NewMiscContextHandler(config.GetWebAppDomain()), -2)

	// start command group
	myBot.AddHandlerToGroup(tgbotHandlers.NewMessage(message.Equal("/start"), handlers.Hello), 0)

	// terms and conditions group
	myBot.AddHandlerToGroup(handlers.NewTermsAndConditionsHandler(), 1)

	myBot.Run(static.ServeStaticContent(config, myBot.AcceptTermsAndConditions))
}
