package bot

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	// MyExampleConfig is just an example config key
	// todo: adjust this
	MyExampleConfig string = "my_example_config"
)

type Settings struct {
	// groupChats is notified of every chat ID stored here, so the rate limiter can
	// route calls to those chats through the group limiter.
	groupChats sync.Map
}

// NewSettings returns a settings store that registers the channel and group IDs it is
// given with groupChats. A nil groupChats is allowed and simply skips that
// registration, which is useful when the caller only needs to read the IDs back.
func NewSettings() *Settings {
	return &Settings{}
}

// RegisterGroupChat registers a group chat ID with the settings store.
// There is no method for unregistering a chat ID, because it does not make sense to forget
// group chat IDs.
func (settings *Settings) RegisterGroupChat(chatID int64) {
	if _, ok := settings.groupChats.Load(chatID); !ok {
		settings.groupChats.Store(chatID, struct{}{})
	}
}

// IsGroupChat returns true if the given chat ID is registered as a group chat.
// It is mostly used for rate-limiting because telegram API requires certain rate-limits
// on the number of requests per second for a single group.
func (settings *Settings) IsGroupChat(chatID int64) bool {
	_, ok := settings.groupChats.Load(chatID)
	return ok
}

// loadSettings reads the persisted config rows into settings. An empty configs table
// is not an error — the bot simply has no channel or group wired up yet.
func (settings *Settings) loadSettings(dbPool *pgxpool.Pool) error {
	rows, err := dbPool.Query(
		context.Background(),
		//language=SQL
		"select key, value from configs where deleted_at is null",
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to get config items from database: %w", err)
	}

	confItems, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[tables.Config])
	if err != nil {
		return fmt.Errorf("failed to collect config items from returned rows: %w", err)
	}

	for _, item := range confItems {
		switch item.Key {
		case MyExampleConfig:
			// todo: do something here
			logger.Log.Debug(
				"loading my example config here",
				zap.String("key", item.Key),
				zap.String("value", item.Value),
			)
		}
	}

	return nil
}
