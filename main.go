package main

import (
	tgbotHandlers "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"my-telegram-bot/configuration"
	"my-telegram-bot/handlers"
	"my-telegram-bot/handlers/context"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/internals/gin_server"
)

func main() {
	config := configuration.Configure()
	myBot := bot.NewBot(config)
	srv := gin_server.NewGinServer(config)

	// enrich context data
	myBot.AddHandlerToGroup(context.NewUserContextHandler(myBot.UsersCache), -1)
	myBot.AddHandlerToGroup(context.NewMiscContextHandler(config.GetWebAppDomain()), -2)

	// terms and conditions group
	myBot.AddHandlerToGroup(handlers.NewTermsAndConditionsHandler(myBot, srv), 0)

	// start command group
	myBot.AddHandlerToGroup(tgbotHandlers.NewMessage(message.Equal("/start"), handlers.Hello), 2)
	myBot.AddHandlerToGroup(tgbotHandlers.NewMessage(message.Equal("/my_id"), handlers.MyID), 2)

	// start bot
	myBot.Run(srv.RunServer)
}
