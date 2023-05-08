package bot

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"
	"my-telegram-bot/pkg/chat"
	"net/http"
	"time"
)

type MyBot struct {
	bot        *gotgbot.Bot
	updater    *ext.Updater
	dispatcher *ext.Dispatcher
	limiter    *rate.Limiter
	chats      map[int64]*chat.Chat
}

func NewBot() *MyBot {
	b, err := gotgbot.NewBot(viper.GetString("token"), &gotgbot.BotOpts{
		Client: http.Client{},
		DefaultRequestOpts: &gotgbot.RequestOpts{
			Timeout: gotgbot.DefaultTimeout,
			APIURL:  gotgbot.DefaultAPIURL,
		},
	})

	if err != nil {
		log.Fatal().Msg("failed to create new bot: " + err.Error())
	}

	// Create updater and dispatcher.
	updater := ext.NewUpdater(&ext.UpdaterOpts{
		Dispatcher: ext.NewDispatcher(&ext.DispatcherOpts{
			// If an error is returned by a handler, log it and continue going.
			Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
				log.Warn().Msg("an error occurred while handling update: " + err.Error())
				return ext.DispatcherActionNoop
			},
			Panic: func(b *gotgbot.Bot, ctx *ext.Context, r interface{}) {
				log.Warn().Msg("a panic occurred while handling update: " + fmt.Sprint(r))
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		}),
	})
	dispatcher := updater.Dispatcher

	return &MyBot{
		bot:        b,
		updater:    updater,
		dispatcher: dispatcher,
		limiter:    rate.NewLimiter(rate.Every(33*time.Millisecond), 1),
		chats:      make(map[int64]*chat.Chat),
	}
}

func (b *MyBot) AddHandler(h ext.Handler) {
	b.dispatcher.AddHandler(h)
}

func (b *MyBot) Start() {
	// Start the webhook server. We start the server before we set the webhook itself, so that when telegram starts
	// sending updates, the server is already ready.
	webhookOpts := ext.WebhookOpts{
		ListenAddr:  fmt.Sprintf("localhost:%d", viper.GetInt("webhook_port")),
		SecretToken: viper.GetString("webhook_secret"),
	}

	err := b.updater.StartWebhook(b.bot, viper.GetString("token"), webhookOpts)
	if err != nil {
		log.Warn().Msg("failed to start webhook: " + err.Error())
	}

	err = b.updater.SetAllBotWebhooks(viper.GetString("webhook_domain"), &gotgbot.SetWebhookOpts{
		MaxConnections:     100,
		DropPendingUpdates: true,
		SecretToken:        webhookOpts.SecretToken,
	})
	if err != nil {
		log.Warn().Msg("failed to set webhook: " + err.Error())
	}

	log.Info().Msgf("%s has been started...\n", b.bot.User.Username)

	// Idle, to keep updates coming in, and avoid bot stopping.
	b.updater.Idle()
}