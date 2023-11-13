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

func (tgUser *TgUserContainer) GetRaw() *tables.TelegramUser {
	return tgUser.user
}

func (tgUser *TgUserContainer) Get(effectiveUser *gotgbot.User) (*tables.TelegramUser, bool) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()

	userDetailsHaveChanged := tgUser.user.FirstName != effectiveUser.FirstName ||
		tgUser.user.LastName != effectiveUser.LastName ||
		tgUser.user.LanguageCode != effectiveUser.LanguageCode

	if tgUser.user.Username.Valid && effectiveUser.Username == "" {
		userDetailsHaveChanged = true
		tgUser.user.Username.Valid = false
		tgUser.user.Username.String = ""
	}

	if !tgUser.user.Username.Valid && effectiveUser.Username != "" {
		userDetailsHaveChanged = true
		tgUser.user.Username.Valid = true
		tgUser.user.Username.String = effectiveUser.Username
	}

	if userDetailsHaveChanged {
		tgUser.user.FirstName = effectiveUser.FirstName
		tgUser.user.LastName = effectiveUser.LastName
		tgUser.user.Username.String = effectiveUser.Username
		if len(tgUser.user.Username.String) > 0 {
			tgUser.user.Username.Valid = true
		} else {
			tgUser.user.Username.Valid = false
		}
		tgUser.user.LanguageCode = effectiveUser.LanguageCode
	}

	return tgUser.user, userDetailsHaveChanged
}

// TermsAndConditionsAccepted method modifies the underlying user object
func (tgUser *TgUserContainer) TermsAndConditionsAccepted(acceptedOn time.Time) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()
	tgUser.user.AcceptedTermsAndConditionsOn.Time = acceptedOn
	tgUser.user.AcceptedTermsAndConditionsOn.Valid = true
	tgUser.user.AcceptedLatestTermsAndConditions = true
}
