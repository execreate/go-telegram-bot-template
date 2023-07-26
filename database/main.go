package main

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"my-telegram-bot/configuration"
	"my-telegram-bot/database/tables"
	myLogger "my-telegram-bot/internals/logger"
)

// This is a helper script for automatically creating DB tables
func main() {
	config := configuration.Configure()

	db, err := gorm.Open(sqlite.Open(config.GetDbDSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		myLogger.LogFatal(err, "failed to connect to database")
	}
	_ = db.AutoMigrate(&tables.TelegramUser{})
}
