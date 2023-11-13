package main

import (
	tgbotHandlers "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"my-telegram-bot/configuration"
	"my-telegram-bot/handlers"
	"my-telegram-bot/handlers/context"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/internals/gin_server"
)

func main() {
	requiredConfig := []string{
		"token",
		"webhook_domain",
		"webhook_port",
		"webapp_domain",
		"webapp_port",
		"webhook_secret",
		"static_content_path",
		"db_dsn",
		"redis_addr",
		"redis_user",
		"redis_pass",
	}
	config := configuration.Configure(requiredConfig)
	myBot := bot.NewBot(config)
	defer myBot.CleanUp()

	srv := gin_server.NewGinServer(config)

	// enrich context data
	myBot.AddHandlerToGroup(context.NewUserContextHandler(myBot.UsersCache), -1)
	myBot.AddHandlerToGroup(context.NewMiscContextHandler(config.GetWebAppDomain()), -2)

	// terms and conditions group
	myBot.AddHandlerToGroup(handlers.NewTermsAndConditionsHandler(myBot, srv), 0)

	// standalone commands group
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("start", handlers.Hello), 2)
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("my_id", handlers.MyID), 2)

	// start bot
	myBot.Run(srv.RunServer)
}
