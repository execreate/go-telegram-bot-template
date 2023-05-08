# Telegram Bot template

This is a template for a Telegram Bot written in Go. It uses a
[code-generated wrapper](https://github.com/PaulSonOfLars/gotgbot) to interact with the Telegram Bot API.

Before you can use this template, you need to create a bot. To do this, you need to talk to
[BotFather](https://t.me/BotFather). Then take the token you get from BotFather and put it in the `config.yaml` file.
Keep your token in a safe place and don't commit it into git ;) Check out the [example configuration](config.dist.yaml)
for a complete list of all available configuration options. Some of them are strictly required, others are optional,
check [config.go](config.go) for details.
