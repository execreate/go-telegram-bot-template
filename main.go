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
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	_shutdownPeriod     = 15 * time.Second
	_shutdownHardPeriod = 3 * time.Second
)

func main() {
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
	config := configuration.Configure(requiredConfig)
	myBot := bot.NewBot(config)

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
	myBot.AddHandlerToGroup(handlers.NewTermsAndConditionsHandler(myBot, srv), 0)

	// standalone commands group
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("start", handlers.Hello), 2)
	myBot.AddHandlerToGroup(tgbotHandlers.NewCommand("my_id", handlers.MyID), 2)

	// graceful shutdown
	// Ensure in-flight requests aren't canceled immediately on SIGTERM
	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())
	server := srv.GetServer(ongoingCtx)
	go func() {
		logger.Log.Info("Gin server starting")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Panic("unexpected error!", zap.Error(err))
		}
	}()

	// start bot
	myBot.Run()

	// wait for shutdown signal
	<-rootCtx.Done()
	stop()
	logger.Log.Info("Received shutdown signal, shutting down.")

	// clean up everything
	go myBot.CleanUp(_shutdownPeriod)
	go logger.Flush()

	// graceful shutdown period setup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownPeriod)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	stopOngoingGracefully()

	if err != nil {
		logger.Log.Info(
			"Failed to wait for ongoing requests to finish, waiting for forced cancellation.",
			zap.Duration("timeout", _shutdownHardPeriod),
		)
		time.Sleep(_shutdownHardPeriod)
	}

	logger.Log.Info("Graceful shutdown complete.")
}
