package tables

import (
	"gorm.io/gorm"
	"time"
)

type TelegramUser struct {
	gorm.Model

	ID           int64  `gorm:"primaryKey;autoIncrement:false"`
	FirstName    string `gorm:"size:250"`
	LastName     string `gorm:"size:250"`
	Username     string `gorm:"size:250;uniqueIndex"`
	LanguageCode string `gorm:"size:3"`
	IsAdmin      bool

	AcceptedTermsAndConditionsOn     *time.Time
	AcceptedLatestTermsAndConditions bool
}

func (u *TelegramUser) FullName() string {
	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}
	return name
}
