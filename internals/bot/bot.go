package bot

import (
	"context"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/internals/commands"
	"my-telegram-bot/internals/logger"
	"my-telegram-bot/internals/users_cache"
	"net/http"
	"strconv"
	"time"
)

type Config interface {
	GetToken() string
	GetWebhookDomain() string
	GetWebhookPath() string
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
	webhookPort       int
	webhookSecret     string
	webAppPort        int
	staticContentPath string
}

func NewBot(config Config) *MyBot {
	b, err := gotgbot.NewBot(config.GetToken(), &gotgbot.BotOpts{
		Client: http.Client{},
		DefaultRequestOpts: &gotgbot.RequestOpts{
			Timeout: time.Second * 60,
			APIURL:  gotgbot.DefaultAPIURL,
		},
	})

	if err != nil {
		logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to create new bot")
	}

	b.UseMiddleware(rateLimiterMiddleware)

	// Create updater and dispatcher.
	updater := ext.NewUpdater(&ext.UpdaterOpts{
		Dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{
			// If an error is returned by a handler, log it and continue going.
			Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
				logger.Log.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("an error occurred while handling update")
				return ext.DispatcherActionEndGroups
			},
			Panic: func(b *gotgbot.Bot, ctx *ext.Context, r interface{}) {
				logger.Log.Error().Any("panic_reason", r).Msg("a panic occurred while handling update")
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		}),
	})
	dispatcher := updater.Dispatcher

	dbPool, err := pgxpool.New(context.Background(), config.GetDbDSN())
	if err != nil {
		logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to connect to database")
	}

	usersCache := users_cache.NewTgUsersCache(dbPool, 4*time.Hour, 4*24*time.Hour)
	settings := &Settings{}

	if rows, err := dbPool.Query(
		context.Background(),
		//language=SQL
		"select key, value from configs where deleted_at is null",
	); err == nil {
		confItems, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[tables.Config])
		if err != nil {
			logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to collect config items from returned rows")
		}

		if confItems != nil {
			for _, item := range confItems {
				switch item.Key {
				case tables.MyChannelID:
					if i, err := strconv.ParseInt(item.Value, 10, 64); err == nil {
						settings.SetMyChannelID(i)
					} else {
						logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Str(
							"value", item.Value).Msg("failed to convert value to integer")
					}
				case tables.MyGroupID:
					if i, err := strconv.ParseInt(item.Value, 10, 64); err == nil {
						settings.SetMyGroupID(i)
					} else {
						logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Str(
							"value", item.Value).Msg("failed to convert value to integer")
					}
				}
			}
		}
	} else {
		if !errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to get config items from database")
		}
	}

	if rows, err := dbPool.Query(
		context.Background(),
		//language=SQL
		`select id,
       		is_admin,
       		language_code
		from telegram_users
        where deleted_at is null and is_admin`,
	); err == nil {
		specialUsers, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[tables.TelegramUser])
		if err != nil {
			logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg(
				"failed to collect telegram users from returned rows")
		}

		ticker := time.NewTicker(time.Millisecond * 50)
		defer ticker.Stop()
		for _, val := range commands.GetCommands(specialUsers) {
			<-ticker.C
			if success, err := b.SetMyCommands(val.Commands, val.Opts); err != nil || !success {
				logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg(
					"failed to set commands")
			}
		}
	} else {
		if !errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg(
				"failed to get users from database")
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
		webhookPort:       config.GetWebhookPort(),
		webhookSecret:     config.GetWebhookSecret(),
		webAppPort:        config.GetWebAppPort(),
		staticContentPath: config.GetStaticContentPath(),
	}
}

// AddHandler adds a new handler to the dispatcher
func (b *MyBot) AddHandler(h ext.Handler) {
	b.dispatcher.AddHandler(h)
}

// AddHandlerToGroup adds a new handler to the specified handler group in dispatcher
func (b *MyBot) AddHandlerToGroup(h ext.Handler, group int) {
	b.dispatcher.AddHandlerToGroup(h, group)
}

// SendMessage sends a message with specified parameters
func (b *MyBot) SendMessage(chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	return b.bot.SendMessage(chatId, text, opts)
}

// EditMessageText edits a message with specified parameters
func (b *MyBot) EditMessageText(text string, opts *gotgbot.EditMessageTextOpts) (*gotgbot.Message, bool, error) {
	return b.bot.EditMessageText(text, opts)
}

// SendDocument sends a document with specified parameters
func (b *MyBot) SendDocument(chatId int64, document gotgbot.InputFile, opts *gotgbot.SendDocumentOpts) (*gotgbot.Message, error) {
	return b.bot.SendDocument(chatId, document, opts)
}

// SendPhoto sends a photo with specified parameters
func (b *MyBot) SendPhoto(chatId int64, photo gotgbot.InputFile, opts *gotgbot.SendPhotoOpts) (*gotgbot.Message, error) {
	return b.bot.SendPhoto(chatId, photo, opts)
}

// SendMediaGroup sends a media group with specified parameters
func (b *MyBot) SendMediaGroup(chatId int64, media []gotgbot.InputMedia, opts *gotgbot.SendMediaGroupOpts) ([]gotgbot.Message, error) {
	return b.bot.SendMediaGroup(chatId, media, opts)
}

// AnswerWebAppQuery answers the web app query
func (b *MyBot) AnswerWebAppQuery(
	webAppQueryId string,
	result gotgbot.InlineQueryResult,
	opts *gotgbot.AnswerWebAppQueryOpts,
) (
	*gotgbot.SentWebAppMessage,
	error,
) {
	return b.bot.AnswerWebAppQuery(webAppQueryId, result, opts)
}

// Run starts webhook server and blocks with updater.Idle()
func (b *MyBot) Run(serveGinServer func()) {
	// Start the server before we set the webhook itself, so that when telegram starts
	// sending updates, the server is already ready.
	webhookOpts := ext.WebhookOpts{
		ListenAddr:  fmt.Sprintf("localhost:%d", b.webhookPort),
		SecretToken: b.webhookSecret,
	}

	err := b.updater.StartWebhook(b.bot, b.webhookPath, webhookOpts)
	if err != nil {
		logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to start webhook")
	}

	err = b.updater.SetAllBotWebhooks(b.webhookDomain, &gotgbot.SetWebhookOpts{
		MaxConnections:     100,
		DropPendingUpdates: true,
		SecretToken:        webhookOpts.SecretToken,
	})
	if err != nil {
		logger.Log.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to set webhook")
	}

	logger.Log.Info().Str("username", b.bot.User.Username).Msg("Webhooks have been started...")

	if serveGinServer != nil {
		serveGinServer()
	} else {
		// Idle, to keep updates coming in, and avoid bot stopping.
		b.updater.Idle()
	}
}

// CleanUp cleans up bot resources
func (b *MyBot) CleanUp() {
	b.DB.Close()
	if _, err := b.bot.DeleteWebhook(&gotgbot.DeleteWebhookOpts{
		DropPendingUpdates: true,
	}); err != nil {
		logger.Log.Warn().Msg("failed to delete the webhook")
	}
}

// GetUsername returns the bot username
func (b *MyBot) GetUsername() string {
	return b.bot.User.Username
}

// GetChat returns the chat with the specified id
func (b *MyBot) GetChat(id int64) (*gotgbot.Chat, error) {
	return b.bot.GetChat(id, nil)
}
