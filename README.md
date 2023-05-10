# Telegram Bot template

This is a template for a Telegram Bot written in Go. It uses a
[code-generated wrapper](https://github.com/PaulSonOfLars/gotgbot) to interact with the Telegram Bot API.

## Start coding

1. Talk to [BotFather](https://t.me/BotFather) and create a bot. You will get a token, keep it safe.
2. For development purposes, we recommend running this with a tool such as ngrok.
Simply [install ngrok](https://ngrok.com/download), make an account on the website, and run `ngrok http 8080`.
3. Clone this repository.
4. Copy `config.dist.yaml` to `config.yaml`, fill in your bot token and other parameters.
5. Run `go run .` to start the bot.
6. Then, simply send `/start` to your bot; if it replies, you've successfully set up your bot!
