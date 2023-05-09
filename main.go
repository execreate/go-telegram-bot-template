package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"my-telegram-bot/pkg/bot"
	"my-telegram-bot/pkg/processors"
)

// This bot repeats everything you say and uses webhooks instead of long polling.
// Webhooks are slightly more complex to run, since they require a running webserver, as well as an HTTPS domain.
// For development purposes, we recommend running this with a tool such as ngrok (https://ngrok.com/).
// Simply install ngrok, make an account on the website, and run:
// `ngrok http 8080`
// Then, copy-paste the HTTPS URL obtained from ngrok (changes every time you run it), and run the following command:
// `go run .`
// Then, simply send /start to your bot; if it replies, you've successfully set up webhooks!
func main() {
	configure()

	myBot := bot.NewBot()

	// Add echo handler to reply to all text messages.
	myBot.AddHandler(handlers.NewMessage(message.Text, processors.Echo))

	myBot.Start()
}
