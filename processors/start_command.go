package processors

import (
	"errors"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/locale"
)

type UserLogger struct {
	dBConn *gorm.DB
}

func NewUserLogger(dbConn *gorm.DB) *UserLogger {
	return &UserLogger{dBConn: dbConn}
}

// Hello replies to a message with "hello world" and the ID of Telegram User.
func (ul *UserLogger) Hello(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveUser
	// Check if the user is already in the database.
	var telegramUser tables.TelegramUser
	err := ul.dBConn.Where("id = ?", user.Id).First(&telegramUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create a new user.
			telegramUser = tables.TelegramUser{
				ID:           user.Id,
				FirstName:    user.FirstName,
				LastName:     user.LastName,
				Username:     user.Username,
				LanguageCode: user.LanguageCode,
			}
			err = ul.dBConn.Create(&telegramUser).Error
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	texts, err := locale.GetTranslations("en")
	if err != nil {
		return err
	}
	_, err = ctx.EffectiveMessage.Reply(
		b,
		fmt.Sprintf("%s (ID: %d)", texts.GetString("hello"), telegramUser.ID),
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}
