package bot

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"my-telegram-bot/internals/users_cache"
	"my-telegram-bot/mylogger"
	"net/http"
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
			Timeout: time.Second * 15,
			APIURL:  gotgbot.DefaultAPIURL,
		},
	})

	if err != nil {
		mylogger.LogFatal(err, "failed to create new bot")
	}

	b.UseMiddleware(rateLimiterMiddleware)

	// Create updater and dispatcher.
	updater := ext.NewUpdater(&ext.UpdaterOpts{
		Dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{
			// If an error is returned by a handler, log it and continue going.
			Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
				mylogger.LogError(err, "an error occurred while handling update")
				return ext.DispatcherActionEndGroups
			},
			Panic: func(b *gotgbot.Bot, ctx *ext.Context, r interface{}) {
				mylogger.LogPanic(r, "a panic occurred while handling update")
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		}),
	})
	dispatcher := updater.Dispatcher

	db, err := gorm.Open(sqlite.Open(config.GetDbDSN()), &gorm.Config{})
	if err != nil {
		mylogger.LogFatal(err, "failed to connect to database")
	}

	usersCache := users_cache.NewTgUsersCache(db, 4*time.Hour, 4*24*time.Hour)

	return &MyBot{
		UsersCache: usersCache,

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

// Run starts webhook server and blocks with updater.Idle()
func (b *MyBot) Run(serveStaticContentFunc func()) {
	// Start the server before we set the webhook itself, so that when telegram starts
	// sending updates, the server is already ready.
	webhookOpts := ext.WebhookOpts{
		ListenAddr:  fmt.Sprintf("localhost:%d", b.webhookPort),
		SecretToken: b.webhookSecret,
	}

	err := b.updater.StartWebhook(b.bot, b.webhookPath, webhookOpts)
	if err != nil {
		mylogger.LogFatal(err, "failed to start webhook")
	}

	err = b.updater.SetAllBotWebhooks(b.webhookDomain, &gotgbot.SetWebhookOpts{
		MaxConnections:     100,
		DropPendingUpdates: true,
		SecretToken:        webhookOpts.SecretToken,
	})
	if err != nil {
		mylogger.LogFatal(err, "failed to set webhook")
	}

	mylogger.LogInfof("Webhooks for %s have been started...", b.bot.User.Username)

	if serveStaticContentFunc != nil {
		serveStaticContentFunc()
	} else {
		// Idle, to keep updates coming in, and avoid bot stopping.
		b.updater.Idle()
	}
}
