package bot

import (
	"context"
	"encoding/json"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"my-telegram-bot/internals/chat"
	"my-telegram-bot/internals/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rateLimitingBotClient middleware wraps the existing BotClient to add a new behavior.
type rateLimitingBotClient struct {
	// Inline existing client to call, allowing us to chain middlewares.
	// Inlining also avoids us having to redefine helper methods part of the interface.
	gotgbot.BotClient
	chats   map[int64]*chat.Chat
	chatsMu *sync.RWMutex
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
		if err := b.waitChatLimiter(ctx, chatIDInt64); err != nil {
			logger.LogError(err, "failed to wait for chat rate limiter")
			return nil, err
		}
	}

	// Allow sending the message without a reply
	if strings.HasPrefix(method, "send") || method == "copyMessage" {
		params["allow_sending_without_reply"] = "true"
	}

	// Call the next bot client instance in the middleware chain.
	return b.BotClient.RequestWithContext(ctx, method, params, data, opts)
}

func (b *rateLimitingBotClient) waitChatLimiter(ctx context.Context, chatID int64) error {
	b.chatsMu.RLock()
	defer b.chatsMu.RUnlock()
	c, ok := b.chats[chatID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		c = chat.NewChat()
		b.chats[chatID] = c
	}
	return c.WaitLimiter(ctx)
}

func (b *rateLimitingBotClient) removeStaleChats() {
	b.chatsMu.Lock()
	defer b.chatsMu.Unlock()
	for chatID, c := range b.chats {
		if c.IsStale() {
			delete(b.chats, chatID)
		}
	}
}

// rateLimiterMiddleware is a simple method that we use to wrap the existing middleware with our new one.
func rateLimiterMiddleware(b gotgbot.BotClient) gotgbot.BotClient {
	c := &rateLimitingBotClient{
		b,
		make(map[int64]*chat.Chat),
		&sync.RWMutex{},
	}
	go func() {
		// Every 24 hours, check for stale chats and remove them from the map.
		ticker := time.NewTicker(time.Hour * 24)
		for range ticker.C {
			c.removeStaleChats()
		}
	}()
	return c
}
