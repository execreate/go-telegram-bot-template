package tables

import (
	"gorm.io/gorm"
)

type TelegramUser struct {
	gorm.Model
	ID           int64  `gorm:"primaryKey;autoIncrement:false"`
	FirstName    string `gorm:"size:250"`
	LastName     string `gorm:"size:250"`
	Username     string `gorm:"size:250"`
	LanguageCode string `gorm:"size:3"`
	IsAdmin      bool

	HasAcceptedTermsAndConditions       bool
	HasAcceptedLatestTermsAndConditions bool
}
