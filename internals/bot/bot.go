package bot

import (
	"context"
	"fmt"
	"time"

	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/commands"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Config interface {
	GetToken() string
	GetWebhookDomain() string
	GetWebhookPath() string
	GetWebhookListenAddr() string
	GetWebhookPort() int
	GetWebhookSecret() string
	GetWebAppPort() int
	GetStaticContentPath() string
	GetDbDSN() string
}

type MyBot struct {
	UsersCache *users_cache.TgUsersCache
	DB         *pgxpool.Pool
	Settings   *Settings

	bot        *gotgbot.Bot
	updater    *ext.Updater
	dispatcher *ext.Dispatcher

	token             string
	webhookDomain     string
	webhookPath       string
	webhookListenAddr string
	webhookPort       int
	webhookSecret     string
	webAppPort        int
	staticContentPath string
}

// NewBot builds the bot, its dispatcher, and the database-backed state it needs.
// Every failure here is returned rather than fatal, so main.go stays the only place
// that ends the process.
func NewBot(config Config, supportedLanguages []string) (*MyBot, error) {
	dbPool, err := pgxpool.New(context.Background(), config.GetDbDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	settings := NewSettings()
	if err := settings.loadSettings(dbPool); err != nil {
		dbPool.Close()
		return nil, err
	}

	botClient, err := newRateLimiterMiddleware(settings)
	if err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("failed to create the rate limiting bot client: %w", err)
	}

	b, err := gotgbot.NewBot(config.GetToken(), &gotgbot.BotOpts{
		BotClient: botClient,
	})
	if err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("failed to create new bot: %w", err)
	}

	// Create updater and dispatcher.
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			logger.Log.Error("an error occurred while handling update", zap.Error(err))
			return ext.DispatcherActionEndGroups
		},
		MaxRoutines: ext.DefaultMaxRoutines,
		Logger:      logger.Slog,
	})
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{Logger: logger.Slog})

	usersCache := users_cache.NewTgUsersCache(dbPool, 4*time.Hour, 4*24*time.Hour)

	for _, language := range supportedLanguages {
		if err := publishCommands(b, dbPool, language); err != nil {
			dbPool.Close()
			return nil, err
		}
	}

	return &MyBot{
		UsersCache: usersCache,
		Settings:   settings,
		DB:         dbPool,

		bot:        b,
		updater:    updater,
		dispatcher: dispatcher,

		token:             config.GetToken(),
		webhookDomain:     config.GetWebhookDomain(),
		webhookPath:       config.GetWebhookPath(),
		webhookListenAddr: config.GetWebhookListenAddr(),
		webhookPort:       config.GetWebhookPort(),
		webhookSecret:     config.GetWebhookSecret(),
		webAppPort:        config.GetWebAppPort(),
		staticContentPath: config.GetStaticContentPath(),
	}, nil
}

// publishCommands pushes the command list to Telegram: the general set for all private
// chats, plus a per-admin set scoped to that admin's chat.
func publishCommands(b *gotgbot.Bot, dbPool *pgxpool.Pool, language string) error {
	rows, err := dbPool.Query(
		context.Background(),
		//language=SQL
		`select id, is_admin, is_owner, language_code
		from telegram_users
        where deleted_at is null and (is_admin or is_owner)`,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to get users from database: %w", err)
	}

	specialUsers, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[tables.TelegramUser])
	if err != nil {
		return fmt.Errorf("failed to collect telegram users from returned rows: %w", err)
	}

	commandsList, err := commands.GetCommands(specialUsers, language)
	if err != nil {
		return fmt.Errorf("failed to build bot commands: %w", err)
	}

	// The calls are spaced out so the burst does not trip Telegram's own rate limits.
	ticker := time.NewTicker(time.Millisecond * 50)
	defer ticker.Stop()
	for _, val := range commandsList {
		<-ticker.C
		if language == locale.FallbackLanguage {
			val.Opts.LanguageCode = ""
		}
		success, err := b.SetMyCommands(val.Commands, val.Opts)
		if err != nil {
			return fmt.Errorf("failed to set commands: %w", err)
		}
		if !success {
			return errors.New("telegram rejected the command list")
		}
	}

	return nil
}

// AddHandler adds a new handler to the dispatcher
func (b *MyBot) AddHandler(h ext.Handler) {
	b.dispatcher.AddHandler(h)
}

// AddHandlerToGroup adds a new handler to the specified handler group in dispatcher
func (b *MyBot) AddHandlerToGroup(h ext.Handler, group int) {
	b.dispatcher.AddHandlerToGroup(h, group)
}

// Bot exposes the underlying gotgbot client so callers can use the full Telegram
// Bot API (SendMessage, SendPoll, EditMessageText, ...) without this package having
// to add a passthrough wrapper for every method.
func (b *MyBot) Bot() *gotgbot.Bot {
	return b.bot
}

// Run starts the webhook server and registers the webhook with Telegram.
func (b *MyBot) Run() error {
	logger.Log.Info("Telegram bot starting")

	webhookOpts := ext.WebhookOpts{
		ListenAddr:  fmt.Sprintf("%s:%d", b.webhookListenAddr, b.webhookPort),
		SecretToken: b.webhookSecret,
	}
	// Start the server before we set the webhook itself, so that when telegram starts
	// sending updates, the server is already ready.
	if err := b.updater.StartWebhook(b.bot, b.webhookPath, webhookOpts); err != nil {
		return fmt.Errorf("failed to start webhook: %w", err)
	}

	// set the webhook
	if err := b.updater.SetAllBotWebhooks(b.webhookDomain, &gotgbot.SetWebhookOpts{
		MaxConnections:     100,
		DropPendingUpdates: false,
		SecretToken:        webhookOpts.SecretToken,
	}); err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	logger.Log.Info("Bot has started", zap.String("username", b.bot.Username))
	return nil
}

// CleanUp cleans up bot resources
func (b *MyBot) CleanUp(shutDownPeriod time.Duration) {
	b.DB.Close()
	if _, err := b.bot.DeleteWebhook(&gotgbot.DeleteWebhookOpts{
		DropPendingUpdates: false,
		RequestOpts:        &gotgbot.RequestOpts{Timeout: shutDownPeriod},
	}); err != nil {
		logger.Log.Warn("failed to delete the webhook", zap.Error(err))
	}
	b.dispatcher.Stop()
	err := b.updater.Stop()
	if err != nil {
		logger.Log.Warn("failed to stop the updater", zap.Error(err))
	}
	logger.Log.Info("Bot has stopped, webhook deleted")
}

// GetUsername returns the bot username
func (b *MyBot) GetUsername() string {
	return b.bot.Username
}
