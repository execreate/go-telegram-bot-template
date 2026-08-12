package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/internals/limiters"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// errUnsupportedChatIDType is returned when chat_id carries a type the middleware
// cannot turn into a chat ID. Callers treat it as "cannot rate limit this request"
// rather than as a request failure.
var errUnsupportedChatIDType = errors.New("unsupported chat_id type")

// parseChatID resolves the chat_id request parameter to an int64. gotgbot stringifies
// params before they reach the middleware chain, but the parameter is typed as any —
// the numeric cases keep rate limiting working if that ever changes.
func parseChatID(value any) (int64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case json.Number:
		return v.Int64()
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("%w: %T", errUnsupportedChatIDType, value)
	}
}

// rateLimitingBotClient middleware wraps the existing BotClient to add a new behavior.
type rateLimitingBotClient struct {
	// Inline existing client to call, allowing us to chain middlewares.
	// Inlining also avoids us having to redefine helper methods part of the interface.
	gotgbot.BotClient
	// settings decide which limiter pool a chat ID is routed to.
	settings            *Settings
	privateChatLimiters *limiters.RateLimiterPool[*limiters.TokenBucketRateLimiter, *limiters.TokenBucketRateLimiterConfig]
	groupChatLimiters   *limiters.RateLimiterPool[*limiters.SlidingWindowRateLimiter, *limiters.SlidingWindowRateLimiterConfig]
}

// RequestWithContext defines a wrapper around the existing RequestWithContext method.
// Note: this is the only method that needs redefining.
// RequestWithContext allows sending a POST request to the telegram bot API with an existing context.
//   - ctx: the timeout contexts to be used.
//   - method: the telegram API method to call.
//   - params: map of parameters to be sent to the telegram API. eg: chat_id, user_id, etc.
//   - data: map of any files to be sent to the telegram API.
//   - opts: request options to use. Note: Timeout opts are ignored when used in RequestWithContext.
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
		chatIDInt64, err := parseChatID(maybeChatID)
		switch {
		case errors.Is(err, errUnsupportedChatIDType):
			// Rate limiting is skipped rather than failing the request, but it must not
			// be skipped silently — that would hide the middleware doing nothing.
			logger.Log.Warn(
				"chat_id has an unexpected type, sending the request without rate limiting",
				zap.String("method", method),
				zap.Any("chat_id", maybeChatID),
				zap.Error(err),
			)
		case err != nil:
			logger.Log.Error("failed to convert chatID to int64", zap.Error(err))
			return nil, err
		case b.settings.IsGroupChat(chatIDInt64):
			if err := b.groupChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
				logger.Log.Error("failed to wait for group chat rate limiter", zap.Error(err))
				return nil, err
			}
		default:
			if err := b.privateChatLimiters.WaitLimiter(ctx, chatIDInt64); err != nil {
				logger.Log.Error("failed to wait for private chat rate limiter", zap.Error(err))
				return nil, err
			}
		}
	}
	// Call the next bot client instance in the middleware chain.
	return b.BotClient.RequestWithContext(ctx, token, method, params, opts)
}

// newRateLimiterMiddleware is to initialize rate-limiting middleware for the bot
// client. Calls to chats in groupChats are routed through the group limiter pool,
// everything else through the private one.
func newRateLimiterMiddleware(settings *Settings) (gotgbot.BotClient, error) {
	privateChatLimiters, err := limiters.NewRateLimiterPool[*limiters.TokenBucketRateLimiter, *limiters.TokenBucketRateLimiterConfig](
		limiters.NewTokenBucketRateLimiter,
		&limiters.TokenBucketRateLimiterConfig{
			Limit: rate.Every(time.Second),
			Burst: 1,
		},
		time.Hour*4,
		time.Hour*24,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create the private chat limiter pool: %w", err)
	}

	groupChatLimiters, err := limiters.NewRateLimiterPool[*limiters.SlidingWindowRateLimiter, *limiters.SlidingWindowRateLimiterConfig](
		limiters.NewSlidingWindowRateLimiter,
		&limiters.SlidingWindowRateLimiterConfig{
			Window: time.Minute,
			MaxN:   20,
		},
		time.Hour*4,
		time.Hour*24,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create the group chat limiter pool: %w", err)
	}

	return &rateLimitingBotClient{
		BotClient: &gotgbot.BaseBotClient{
			Client:             http.Client{},
			UseTestEnvironment: false,
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: gotgbot.DefaultTimeout,
				APIURL:  gotgbot.DefaultAPIURL,
			},
		},
		settings:            settings,
		privateChatLimiters: privateChatLimiters,
		groupChatLimiters:   groupChatLimiters,
	}, nil
}
