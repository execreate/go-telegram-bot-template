package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"my-telegram-bot/configuration"
	"my-telegram-bot/database/tables"
	myLogger "my-telegram-bot/internals/logger"
)

func main() {
	config := configuration.Configure([]string{"db_dsn"})

	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: config.GetDbDSN(), PreferSimpleProtocol: true}),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		},
	)
	if err != nil {
		myLogger.LogFatal(err, "failed to connect to database")
	}
	_ = db.AutoMigrate(
		&tables.TelegramUser{},
	)
}
