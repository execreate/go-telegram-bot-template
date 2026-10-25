package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	tgbotHandlers "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/execreate/go-telegram-bot-template/configuration"
	"github.com/execreate/go-telegram-bot-template/handlers"
	"github.com/execreate/go-telegram-bot-template/handlers/contextual"
	"github.com/execreate/go-telegram-bot-template/internals/bot"
	"github.com/execreate/go-telegram-bot-template/internals/gin_server"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// The worst-case shutdown takes _shutdownPeriod + _shutdownHardPeriod. Keep the sum
// below the orchestrator's grace period (Docker's default is 10s, Kubernetes' 30s),
// or the process is SIGKILLed mid-drain.
const (
	_shutdownPeriod     = 7 * time.Second
	_shutdownHardPeriod = 2 * time.Second
)

var supportedLanguages = []string{"en"}

func main() {
	// Parse the command line before anything reads a flag-backed value. The locale
	// package registers --locale-path but deliberately leaves parsing to us.
	locale.ParseFlags()

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	defer logger.Flush()

	requiredConfig := []string{
		"token",
		"webhook_domain",
		"webhook_port",
		"webapp_domain",
		"webapp_port",
		"webhook_secret",
		"static_content_path",
		"db_dsn",
		"redis_addr",
		"redis_user",
		"redis_pass",
	}
	config, err := configuration.Configure(requiredConfig)
	if err != nil {
		logger.Log.Fatal("failed to load configuration", zap.Error(err))
	}

	// Reconfigure the logger from the resolved config before anything else runs.
	logger.Configure(config.GetDebug())

	myBot, err := bot.NewBot(config, supportedLanguages)
	if err != nil {
		logger.Log.Fatal("failed to create the bot", zap.Error(err))
	}

	if config.GetDebug() {
		logger.Log.Info("Running in debug mode, logging all requests and responses.")
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	srv := gin_server.NewGinServer(config)

	// enrich context data
	myBot.AddHandlerToGroup(contextual.NewUserContextHandler(myBot.UsersCache), -1)
	myBot.AddHandlerToGroup(contextual.NewMiscContextHandler(config.GetWebAppDomain()), -2)

	// terms and conditions group
	termsHandler, err := handlers.NewTermsAndConditionsHandler(
		myBot,
		srv,
		supportedLanguages,
		config.GetTermsAndConditionsVersion(),
	)
	if err != nil {
		logger.Log.Fatal("failed to create the terms and conditions handler", zap.Error(err))
	}
	myBot.AddHandlerToGroup(termsHandler, 0)

	// standalone commands group
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("start", handlers.Hello), 2)
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("my_id", handlers.MyID), 2)

	// graceful shutdown
	// Ensure in-flight requests aren't canceled immediately on SIGTERM
	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())
	server := srv.GetServer(ongoingCtx)

	serverErr := make(chan error, 1)
	go func() {
		logger.Log.Info("Gin server starting")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// start bot
	if err := myBot.Run(); err != nil {
		logger.Log.Fatal("failed to start the bot", zap.Error(err))
	}

	// wait for shutdown signal
	var fatalErr error
	select {
	case <-rootCtx.Done():
		logger.Log.Info("Received shutdown signal, shutting down.")
	case fatalErr = <-serverErr:
		logger.Log.Error("Web app server failed, shutting down.", zap.Error(fatalErr))
	}
	// Restore default signal handling, so a second signal kills the process outright.
	stop()

	// graceful shutdown period setup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownPeriod)
	defer cancel()

	// The bot and the web app server drain in parallel against the same deadline.
	botStopped := make(chan struct{})
	go func() {
		defer close(botStopped)
		myBot.CleanUp()
	}()

	err = server.Shutdown(shutdownCtx)
	stopOngoingGracefully()

	if err != nil {
		logger.Log.Info(
			"Failed to wait for ongoing requests to finish, waiting for forced cancellation.",
			zap.Duration("timeout", _shutdownHardPeriod),
		)
		time.Sleep(_shutdownHardPeriod)
	}

	select {
	case <-botStopped:
	case <-shutdownCtx.Done():
		logger.Log.Warn("Timed out waiting for in-flight bot updates to finish.")
	}

	if fatalErr != nil {
		logger.Log.Fatal("Shut down after a web app server failure.", zap.Error(fatalErr))
	}
	logger.Log.Info("Graceful shutdown complete.")
}
