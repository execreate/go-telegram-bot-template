package tables

import (
	"gorm.io/gorm"
)

type TelegramUser struct {
	gorm.Model
	ID           int64 `gorm:"primaryKey;autoIncrement:false"`
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
}
