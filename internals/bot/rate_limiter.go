package bot

import (
	"context"
	"encoding/json"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"golang.org/x/time/rate"
	"my-telegram-bot/internals/chat"
	"my-telegram-bot/internals/logger"
	"strconv"
	"time"
)

// rateLimitingBotClient middleware wraps the existing BotClient to add a new behavior.
type rateLimitingBotClient struct {
	// Inline existing client to call, allowing us to chain middlewares.
	// Inlining also avoids us having to redefine helper methods part of the interface.
	gotgbot.BotClient
	privateChatLimiters *chat.TokenBucketRateLimiterPool
	groupChatLimiters   *chat.SlidingWindowRateLimiterPool
}

// RequestWithContext defines a wrapper around existing RequestWithContext method.
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
	method string,
	params map[string]string,
	data map[string]gotgbot.NamedReader,
	opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	// if we are interacting with a specific chat_id, we wait for the chat rate limiter.
	if chatID, ok := params["chat_id"]; ok && len(chatID) > 0 {
		chatIDInt64, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			logger.LogError(err, "failed to convert chatID to int64")
			return nil, err
		}
		if GroupChats.IsGroupChat(chatIDInt64) {
			if err := b.groupChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
				logger.LogError(err, "failed to wait for group chat rate limiter")
				return nil, err
			}
		} else {
			if err := b.privateChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
				logger.LogError(err, "failed to wait for private chat rate limiter")
				return nil, err
			}
		}
	}
	// Call the next bot client instance in the middleware chain.
	return b.BotClient.RequestWithContext(ctx, method, params, data, opts)
}

// rateLimiterMiddleware is a simple method that we use to wrap the existing middleware with our new one.
func rateLimiterMiddleware(b gotgbot.BotClient) gotgbot.BotClient {
	return &rateLimitingBotClient{
		BotClient: b,
		privateChatLimiters: chat.NewTokenBucketRateLimiterPool(
			rate.Every(time.Second),
			1,
			time.Hour*4,
			time.Hour*24,
		),
		groupChatLimiters: chat.NewSlidingWindowRateLimiterPool(
			time.Minute,
			20,
			time.Hour*4,
			time.Hour*24,
		),
	}
}
