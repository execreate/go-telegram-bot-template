package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/internals/limiters"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// rateLimitingBotClient middleware wraps the existing BotClient to add a new behavior.
type rateLimitingBotClient struct {
	// Inline existing client to call, allowing us to chain middlewares.
	// Inlining also avoids us having to redefine helper methods part of the interface.
	gotgbot.BotClient
	privateChatLimiters *limiters.TokenBucketRateLimiterPool
	groupChatLimiters   *limiters.SlidingWindowRateLimiterPool
}

// RequestWithContext defines a wrapper around the existing RequestWithContext method.
// Note: this is the only method that needs redefining.
// RequestWithContext allows sending a POST request to the telegram bot API with an existing context.
//   - ctx: the timeout contexts to be used.
//   - method: the telegram API method to call.
//   - params: map of parameters to be sending to the telegram API. eg: chat_id, user_id, etc.
//   - data: map of any files to be sending to the telegram API.
//   - opts: request opts to use. Note: Timeout opts are ignored when used in RequestWithContext.
//     Timeout handling is the responsibility of the caller/context owner.
func (b *rateLimitingBotClient) RequestWithContext(
	ctx context.Context,
	token string,
	method string,
	params map[string]any,
	opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	// if we are interacting with a specific chat_id, we wait for the chat rate limiter.
	if maybeChatID, ok := params["chat_id"]; ok {
		if chatID, ok := maybeChatID.(string); ok {
			chatIDInt64, err := strconv.ParseInt(chatID, 10, 64)
			if err != nil {
				logger.Log.Error("failed to convert chatID to int64", zap.Error(err))
				return nil, err
			}
			if GroupChats.IsGroupChat(chatIDInt64) {
				if err := b.groupChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
					logger.Log.Error("failed to wait for group chat rate limiter", zap.Error(err))
					return nil, err
				}
			} else {
				if err := b.privateChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
					logger.Log.Error("failed to wait for private chat rate limiter", zap.Error(err))
					return nil, err
				}
			}
		}
	}
	// Call the next bot client instance in the middleware chain.
	return b.BotClient.RequestWithContext(ctx, token, method, params, opts)
}

// newRateLimiterMiddleware is to initialize rate-limiting middleware for the bot client.
func newRateLimiterMiddleware() gotgbot.BotClient {
	return &rateLimitingBotClient{
		BotClient: &gotgbot.BaseBotClient{
			Client:             http.Client{},
			UseTestEnvironment: false,
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: gotgbot.DefaultTimeout,
				APIURL:  gotgbot.DefaultAPIURL,
			},
		},
		privateChatLimiters: limiters.NewTokenBucketRateLimiterPool(
			rate.Every(time.Second),
			1,
			time.Hour*4,
			time.Hour*24,
		),
		groupChatLimiters: limiters.NewSlidingWindowRateLimiterPool(
			time.Minute,
			20,
			time.Hour*4,
			time.Hour*24,
		),
	}
}
