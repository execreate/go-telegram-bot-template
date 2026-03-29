package users_cache

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache/user_container"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type TgUsersCache struct {
	dbPool         *pgxpool.Pool
	users          map[int64]*user_container.TgUserContainer
	mu             sync.RWMutex
	staleThreshold time.Duration
}

func NewTgUsersCache(dbPool *pgxpool.Pool, cleanUpInterval, staleThreshold time.Duration) *TgUsersCache {
	tgUsrCache := &TgUsersCache{
		dbPool:         dbPool,
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

	rows, _ := tgUsrPool.dbPool.Query(
		context.Background(),
		//language=SQL
		"select * from telegram_users where deleted_at is null and username = $1",
		username,
	)
	defer rows.Close()

	telegramUser, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tables.TelegramUser])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Warn("user with given username not found", zap.String("username", username))
		} else {
			logger.Log.Error(
				"failed to query for a user",
				zap.String("username", username),
				zap.Error(err),
			)
		}
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
	rows, _ := tgUsrPool.dbPool.Query(
		context.Background(),
		//language=SQL
		"select * from telegram_users where deleted_at is null and id = $1",
		userID,
	)
	defer rows.Close()

	telegramUser, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tables.TelegramUser])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Warn("user not found", zap.Int64("user_id", userID))
		} else {
			logger.Log.Error(
				"failed to query for a user",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
		}
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
			go func(db *pgxpool.Pool, user *tables.TelegramUser) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				var err error
				if user.Username.Valid {
					_, err = db.Exec(
						ctx,
						`update telegram_users 
						set first_name = $1, last_name = $2, username = $3, language_code = $4
						where id = $5`,
						user.FirstName,
						user.LastName,
						user.Username,
						user.LanguageCode,
						user.ID,
					)
				} else {
					_, err = db.Exec(
						ctx,
						`update telegram_users 
						set first_name = $1, last_name = $2, username = NULL, language_code = $3
						where id = $4`,
						user.FirstName,
						user.LastName,
						user.LanguageCode,
						user.ID,
					)
				}
				if err != nil {
					logger.Log.Error("failed to update user details", zap.Error(err))
				}
			}(tgUsrPool.dbPool, user)
		}

		return user, nil
	}

	// serve the user from database if it's not in memory
	rows, _ := tgUsrPool.dbPool.Query(
		context.Background(),
		//language=SQL
		"select * from telegram_users where deleted_at is null and id = $1",
		effectiveUser.Id,
	)
	defer rows.Close()

	telegramUser, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tables.TelegramUser])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			now := time.Now()
			nullUsername := sql.NullString{String: effectiveUser.Username, Valid: false}
			if len(nullUsername.String) > 0 {
				nullUsername.Valid = true
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			if _, err := tgUsrPool.dbPool.Exec(
				ctx,
				//language=SQL
				`insert into telegram_users (
                        id,
                        created_at,
                        updated_at,
                        deleted_at,
                        first_name,
                        last_name,
                        username,
                        language_code,
                        is_admin,
                        accepted_terms_and_conditions_on,
                        accepted_terms_and_conditions_version
                    ) values (
						$1,
                    	$2,
						$3,
                    	null,
						$4,
                    	$5,
						$6,
                    	$7,
                    	false,
                    	null,
                    	null
					)`,
				effectiveUser.Id,
				now,
				now,
				effectiveUser.FirstName,
				effectiveUser.LastName,
				nullUsername,
				effectiveUser.LanguageCode,
			); err != nil {
				logger.Log.Error("failed to insert new user details into database", zap.Error(err))
				return nil, err
			}

			telegramUser = tables.TelegramUser{
				SoftDeleteModel: tables.SoftDeleteModel{
					ID:        effectiveUser.Id,
					CreatedAt: now,
					UpdatedAt: now,
				},
				FirstName:    effectiveUser.FirstName,
				LastName:     effectiveUser.LastName,
				Username:     nullUsername,
				LanguageCode: effectiveUser.LanguageCode,
			}
		} else {
			logger.Log.Error("failed to query for a user", zap.Error(err))
			return nil, err
		}
	}

	go tgUsrPool.addNewUser(
		telegramUser.ID,
		user_container.NewTelegramUserContainer(&telegramUser),
	)

	return &telegramUser, nil
}

func (tgUsrPool *TgUsersCache) UserHasAcceptedTermsAndConditions(userID int64, version string) error {
	tgUsrPool.mu.RLock()
	defer tgUsrPool.mu.RUnlock()

	if userContainer, ok := tgUsrPool.users[userID]; ok {
		acceptedOn := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if _, err := tgUsrPool.dbPool.Exec(
			ctx,
			//language=SQL
			`update telegram_users
			set accepted_terms_and_conditions_on = $1, accepted_terms_and_conditions_version = $2
			where id = $3`,
			acceptedOn,
			version,
			userID,
		); err != nil {
			logger.Log.Error(
				"failed to update user details in database",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			return err
		}
		userContainer.TermsAndConditionsAccepted(acceptedOn, version)
		return nil
	}

	return errors.New("user not found in cache, should never come here")
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
