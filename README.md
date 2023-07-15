# Telegram Bot template

This is a template for a Telegram Bot written in Go. It uses a
[code-generated wrapper](https://github.com/PaulSonOfLars/gotgbot) to interact with the Telegram Bot API.

## Start coding

1. Talk to [BotFather](https://t.me/BotFather) and create a bot. You will get a token, keep it safe.
2. For development purposes, we recommend running this with a tool such as ngrok.
Simply [install ngrok](https://ngrok.com/download), copy `ngrok.dist.yaml` to `ngrok.yaml`, set your `authtoken`,
and run `ngrok start --config=ngrok.yaml bot_webhook web_app`.
3. Use this template to create a repo for your bot, clone it to your local dev environment.
4. Copy `config.dist.yaml` to `config.yaml`, fill in your bot token and other parameters.
5. Run `go run .` to start the bot.
6. Then, simply send `/start` to your bot; if it replies, you've successfully set up your bot!

## Features

This setup is currently in use in my other project and will likely be used in other projects as well,
it includes some common things like:

1. Dockerfile
2. Ngrok for easy development
3. WebApp setup with gin-gonic webserver
4. Database setup
5. Handlers to keep common stuff in context
6. Viper for config and translatable text handling
7. In-memory cache to keep users information
8. Rate-limiter for requests to the telegram servers
9. Ready to use Redis storage class for conversation metadata

Just dive into the code and you'll see. Happy coding!
