package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/processors"
)

func main() {
	config := configure()

	myBot := bot.NewBot(config)

	// Add echo handler to reply to all text messages.
	myBot.AddHandler(handlers.NewMessage(message.Text, processors.Echo))

	myBot.Start()
}
