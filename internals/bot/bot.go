package bot

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	DbConn     *gorm.DB
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
		logger.LogFatal(err, "failed to create new bot")
	}

	b.UseMiddleware(rateLimiterMiddleware)

	// Create updater and dispatcher.
	updater := ext.NewUpdater(&ext.UpdaterOpts{
		Dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{
			// If an error is returned by a handler, log it and continue going.
			Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
				logger.LogError(err, "an error occurred while handling update")
				return ext.DispatcherActionEndGroups
			},
			Panic: func(b *gotgbot.Bot, ctx *ext.Context, r interface{}) {
				logger.LogPanic(r, "a panic occurred while handling update")
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		}),
	})
	dispatcher := updater.Dispatcher

	db, err := gorm.Open(postgres.New(postgres.Config{DSN: config.GetDbDSN(), PreferSimpleProtocol: true}))
	if err != nil {
		logger.LogFatal(err, "failed to connect to database")
	}

	usersCache := users_cache.NewTgUsersCache(db, 4*time.Hour, 4*24*time.Hour)
	settings := &Settings{}

	var confItems []tables.Config
	if err := db.Find(&confItems).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogFatal(err, "failed to get config items from database")
		}
	}

	if confItems != nil {
		for _, item := range confItems {
			switch item.Key {
			case tables.MyChannelID:
				if i, err := strconv.ParseInt(item.Value, 10, 64); err == nil {
					settings.SetMyChannelID(i)
				} else {
					logger.LogErrorf(err, "failed to convert %s to integer", item.Value)
				}
			case tables.MyGroupID:
				if i, err := strconv.ParseInt(item.Value, 10, 64); err == nil {
					settings.SetMyGroupID(i)
				} else {
					logger.LogErrorf(err, "failed to convert %s to integer", item.Value)
				}
			}
		}
	}

	var specialUsers []tables.TelegramUser
	if err := db.Where("is_admin").Find(&specialUsers).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogFatal(err, "failed to get users from database")
		}
	}

	chatIds := commands.NewSpecialChatIds()
	for _, user := range specialUsers {
		if user.IsAdmin {
			chatIds.Admins = append(chatIds.Admins, user.ID)
		}
	}

	ticker := time.NewTicker(time.Millisecond * 50)
	defer ticker.Stop()
	for _, val := range commands.GetCommands(chatIds) {
		<-ticker.C
		if success, err := b.SetMyCommands(val.Commands, val.Opts); err != nil || !success {
			logger.LogFatal(err, "failed to set commands")
		}
	}

	return &MyBot{
		UsersCache: usersCache,
		DbConn:     db,
		Settings:   settings,

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
		logger.LogFatal(err, "failed to start webhook")
	}

	err = b.updater.SetAllBotWebhooks(b.webhookDomain, &gotgbot.SetWebhookOpts{
		MaxConnections:     100,
		DropPendingUpdates: true,
		SecretToken:        webhookOpts.SecretToken,
	})
	if err != nil {
		logger.LogFatal(err, "failed to set webhook")
	}

	logger.LogInfof("Webhooks for %s have been started...", b.bot.User.Username)

	if serveGinServer != nil {
		serveGinServer()
	} else {
		// Idle, to keep updates coming in, and avoid bot stopping.
		b.updater.Idle()
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
