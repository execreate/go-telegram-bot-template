package bot

import (
	"my-telegram-bot/mylogger"
)

func (b *MyBot) AcceptTermsAndConditions(userID int64) {
	if err := b.UsersCache.UserHasAcceptedTermsAndConditions(userID); err != nil {
		mylogger.LogError(err, "failed to update user's terms and conditions acceptance status")
		_, err = b.bot.SendMessage(userID, "Failed to accept Terms and Conditions, please try again", nil)
		if err != nil {
			mylogger.LogError(err, "failed to send message to user")
		}
	} else {
		_, err := b.bot.SendMessage(userID, "Thank you for accepting our Terms and Conditions.", nil)
		if err != nil {
			mylogger.LogError(err, "failed to send message to user")
		}
	}
}
