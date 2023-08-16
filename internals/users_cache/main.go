package users_cache

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/internals/logger"
	"my-telegram-bot/internals/users_cache/user_container"
	"strings"
	"sync"
	"time"
)

type TgUsersCache struct {
	dbConn         *gorm.DB
	users          map[int64]*user_container.TgUserContainer
	mu             sync.RWMutex
	staleThreshold time.Duration
}

func NewTgUsersCache(dbConn *gorm.DB, cleanUpInterval, staleThreshold time.Duration) *TgUsersCache {
	tgUsrCache := &TgUsersCache{
		dbConn:         dbConn,
		users:          make(map[int64]*user_container.TgUserContainer),
		staleThreshold: staleThreshold,
	}

	// run a goroutine to clean up the users map
	go tgUsrCache.cleanUpRoutine(cleanUpInterval)

	return tgUsrCache
}

func (tgUsrPool *TgUsersCache) GetByUsername(username string) (*tables.TelegramUser, error) {
	tgUsrPool.mu.RLock()
	defer tgUsrPool.mu.RUnlock()

	if strings.HasPrefix(username, "@") {
		username = username[1:]
	}

	// serve the user from database if it's not in memory
	var telegramUser tables.TelegramUser

	if err := tgUsrPool.dbConn.Where("username = ?", username).First(&telegramUser).Error; err != nil {
		return nil, err
	}

	if _, ok := tgUsrPool.users[telegramUser.ID]; !ok {
		go tgUsrPool.addNewUser(
			telegramUser.ID,
			user_container.NewTelegramUserContainer(&telegramUser),
		)
	}

	return &telegramUser, nil
}

func (tgUsrPool *TgUsersCache) GetByID(userID int64) (*tables.TelegramUser, error) {
	tgUsrPool.mu.RLock()
	defer tgUsrPool.mu.RUnlock()

	// try to serve from memory
	if userContainer, ok := tgUsrPool.users[userID]; ok {
		return userContainer.GetRaw(), nil
	}

	// serve the user from database if it's not in memory
	var telegramUser tables.TelegramUser

	if err := tgUsrPool.dbConn.Where("id = ?", userID).First(&telegramUser).Error; err != nil {
		return nil, err
	}

	if _, ok := tgUsrPool.users[telegramUser.ID]; !ok {
		go tgUsrPool.addNewUser(
			telegramUser.ID,
			user_container.NewTelegramUserContainer(&telegramUser),
		)
	}

	return &telegramUser, nil
}

func (tgUsrPool *TgUsersCache) Get(effectiveUser *gotgbot.User) (*tables.TelegramUser, error) {
	tgUsrPool.mu.RLock()
	defer tgUsrPool.mu.RUnlock()

	// serve the user from memory if it's already there
	if userContainer, ok := tgUsrPool.users[effectiveUser.Id]; ok {
		user, needsUpdate := userContainer.Get(effectiveUser)

		if needsUpdate {
			go func(dbConn *gorm.DB, user *tables.TelegramUser) {
				if err := dbConn.Save(user).Error; err != nil {
					logger.LogError(err, "failed to update user details")
				}
			}(tgUsrPool.dbConn, user)
		}

		return user, nil
	}

	// serve the user from database if it's not in memory
	var telegramUser tables.TelegramUser
	err := tgUsrPool.dbConn.Where("id = ?", effectiveUser.Id).First(&telegramUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			telegramUser = tables.TelegramUser{
				ID:           effectiveUser.Id,
				FirstName:    effectiveUser.FirstName,
				LastName:     effectiveUser.LastName,
				Username:     effectiveUser.Username,
				LanguageCode: effectiveUser.LanguageCode,
			}
			err = tgUsrPool.dbConn.Create(&telegramUser).Error
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	go tgUsrPool.addNewUser(
		telegramUser.ID,
		user_container.NewTelegramUserContainer(&telegramUser),
	)

	return &telegramUser, nil
}

func (tgUsrPool *TgUsersCache) UserHasAcceptedTermsAndConditions(userID int64) error {
	tgUsrPool.mu.RLock()
	defer tgUsrPool.mu.RUnlock()

	if userContainer, ok := tgUsrPool.users[userID]; ok {
		acceptedOn := time.Now()
		err := tgUsrPool.dbConn.Model(&tables.TelegramUser{ID: userID}).Updates(
			map[string]interface{}{
				"accepted_terms_and_conditions_on":     acceptedOn,
				"accepted_latest_terms_and_conditions": true,
			},
		).Error
		if err != nil {
			return err
		}
		userContainer.TermsAndConditionsAccepted(acceptedOn)
		return nil
	} else {
		return errors.New("user not found in cache, should never come here")
	}
}

func (tgUsrPool *TgUsersCache) addNewUser(userID int64, user *user_container.TgUserContainer) {
	tgUsrPool.mu.Lock()
	defer tgUsrPool.mu.Unlock()

	tgUsrPool.users[userID] = user
}

func (tgUsrPool *TgUsersCache) cleanUpRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		tgUsrPool.cleanUpStaleUsers()
	}
}

func (tgUsrPool *TgUsersCache) cleanUpStaleUsers() {
	tgUsrPool.mu.Lock()
	defer tgUsrPool.mu.Unlock()

	for key, usr := range tgUsrPool.users {
		if usr.IsStale(tgUsrPool.staleThreshold) {
			delete(tgUsrPool.users, key)
		}
	}
}
