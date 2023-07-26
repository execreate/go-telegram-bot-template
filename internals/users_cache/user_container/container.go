package user_container

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"my-telegram-bot/database/tables"
	"sync"
	"time"
)

type TgUserContainer struct {
	user         *tables.TelegramUser
	lastActivity time.Time
	mu           sync.RWMutex
}

func NewTelegramUserContainer(user *tables.TelegramUser) *TgUserContainer {
	return &TgUserContainer{
		user:         user,
		lastActivity: time.Now(),
	}
}

func (tgUser *TgUserContainer) IsStale(threshold time.Duration) bool {
	tgUser.mu.RLock()
	defer tgUser.mu.RUnlock()

	if time.Now().Sub(tgUser.lastActivity) > threshold {
		return true
	}

	return false
}

func (tgUser *TgUserContainer) Get(effectiveUser *gotgbot.User) (*tables.TelegramUser, bool) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()

	userDetailsHaveChanged := tgUser.user.FirstName != effectiveUser.FirstName ||
		tgUser.user.LastName != effectiveUser.LastName ||
		tgUser.user.Username != effectiveUser.Username ||
		tgUser.user.LanguageCode != effectiveUser.LanguageCode

	if userDetailsHaveChanged {
		tgUser.user.FirstName = effectiveUser.FirstName
		tgUser.user.LastName = effectiveUser.LastName
		tgUser.user.Username = effectiveUser.Username
		tgUser.user.LanguageCode = effectiveUser.LanguageCode
	}

	return tgUser.user, userDetailsHaveChanged
}

// TermsAndConditionsAccepted method modifies the underlying user object
func (tgUser *TgUserContainer) TermsAndConditionsAccepted(acceptedOn time.Time) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()
	tgUser.user.AcceptedTermsAndConditionsOn = acceptedOn
	tgUser.user.AcceptedLatestTermsAndConditions = true
}
