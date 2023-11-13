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

1. Viper for config handling
2. Dockerfile + Compose
3. Ngrok for easy development
4. Logging
5. WebApp setup
6. Database setup

### Migrations

```shell
goose -dir ./database/migrations/postgres postgres "user=user password=pass dbname=my_db host=localhost port=5437 sslmode=disable" up
```

```shell
goose -s -dir ./database/migrations/postgres create migration_comment sql
```
