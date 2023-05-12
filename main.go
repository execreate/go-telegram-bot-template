package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/mylogger"
	"my-telegram-bot/processors"
)

func main() {
	config := configure()
	myBot := bot.NewBot(config)

	db, err := gorm.Open(sqlite.Open(config.GetString("db_dsn")), &gorm.Config{})
	if err != nil {
		mylogger.LogFatal(err, "failed to connect to database")
	}

	userLoggerInstance := processors.NewUserLogger(db)

	myBot.AddHandler(handlers.NewMessage(message.Equal("/start"), userLoggerInstance.Hello))
	myBot.AddHandler(handlers.NewMessage(message.Text, processors.Echo))

	myBot.Run()
}
