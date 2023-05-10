package bot

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/rs/zerolog/log"
	"my-telegram-bot/internals/mylogger"
	"net/http"
	"time"
)

type Config interface {
	GetToken() string
	GetWebhookDomain() string
	GetWebhookPort() int
	GetWebhookSecret() string
}

type MyBot struct {
	bot        *gotgbot.Bot
	updater    *ext.Updater
	dispatcher *ext.Dispatcher

	token         string
	webhookDomain string
	webhookPort   int
	webhookSecret string
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
				return ext.DispatcherActionNoop
			},
			Panic: func(b *gotgbot.Bot, ctx *ext.Context, r interface{}) {
				log.Panic().Any("panic_reason", r).Msg("a panic occurred while handling update")
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		}),
	})
	dispatcher := updater.Dispatcher

	return &MyBot{
		bot:        b,
		updater:    updater,
		dispatcher: dispatcher,

		token:         config.GetToken(),
		webhookDomain: config.GetWebhookDomain(),
		webhookPort:   config.GetWebhookPort(),
		webhookSecret: config.GetWebhookSecret(),
	}
}

// AddHandler adds a new handler to the dispatcher
func (b *MyBot) AddHandler(h ext.Handler) {
	b.dispatcher.AddHandler(h)
}

// Start starts webhook server and blocks with updater.Idle()
func (b *MyBot) Start() {
	// Start the server before we set the webhook itself, so that when telegram starts
	// sending updates, the server is already ready.
	webhookOpts := ext.WebhookOpts{
		ListenAddr:  fmt.Sprintf("localhost:%d", b.webhookPort),
		SecretToken: b.webhookSecret,
	}

	err := b.updater.StartWebhook(b.bot, b.token, webhookOpts)
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

	mylogger.LogInfof("%s has been started...\n", b.bot.User.Username)

	// Idle, to keep updates coming in, and avoid bot stopping.
	b.updater.Idle()
}
