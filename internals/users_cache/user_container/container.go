package user_container

import (
	"database/sql"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
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

// GetRaw returns a copy of the cached user. A copy (rather than the live pointer)
// is returned so callers can read it without racing the in-place updates that Get
// and TermsAndConditionsAccepted perform under the container lock.
func (tgUser *TgUserContainer) GetRaw() *tables.TelegramUser {
	tgUser.mu.RLock()
	defer tgUser.mu.RUnlock()
	userCopy := *tgUser.user
	return &userCopy
}

func (tgUser *TgUserContainer) Get(effectiveUser *gotgbot.User) (*tables.TelegramUser, bool) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()

	// Telegram sends an empty username when the user has none, which maps to a NULL
	// column. Comparing the whole NullString catches every transition — set, unset,
	// and one username swapped for another.
	username := sql.NullString{
		String: effectiveUser.Username,
		Valid:  effectiveUser.Username != "",
	}

	userDetailsHaveChanged := tgUser.user.FirstName != effectiveUser.FirstName ||
		tgUser.user.LastName != effectiveUser.LastName ||
		tgUser.user.LanguageCode != effectiveUser.LanguageCode ||
		tgUser.user.Username != username

	if userDetailsHaveChanged {
		tgUser.user.FirstName = effectiveUser.FirstName
		tgUser.user.LastName = effectiveUser.LastName
		tgUser.user.Username = username
		tgUser.user.LanguageCode = effectiveUser.LanguageCode
	}

	// Return a copy so the caller never holds the live pointer (see GetRaw).
	userCopy := *tgUser.user
	return &userCopy, userDetailsHaveChanged
}

// TermsAndConditionsAccepted method updates the user's accepted terms and conditions info
func (tgUser *TgUserContainer) TermsAndConditionsAccepted(acceptedOn time.Time, version string) {
	tgUser.mu.Lock()
	defer tgUser.mu.Unlock()

	tgUser.lastActivity = time.Now()
	tgUser.user.AcceptedTermsAndConditionsOn.Time = acceptedOn
	tgUser.user.AcceptedTermsAndConditionsOn.Valid = true
	tgUser.user.AcceptedTermsAndConditionsVersion.String = version
	tgUser.user.AcceptedTermsAndConditionsVersion.Valid = true
}
