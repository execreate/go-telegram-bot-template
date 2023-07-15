package conversations

import (
	"context"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisConfig interface {
	GetRedisAddr() string
	GetRedisUsername() string
	GetRedisPassword() string
}

type ConversationStorage struct {
	// keyStrategy defines how to calculate keys for each conversation.
	keyStrategy conversation.KeyStrategy
	// redisClient keeps redis connection
	redisClient *redis.Client
}

func NewConversationStorage(config RedisConfig) *ConversationStorage {
	conn := redis.NewClient(&redis.Options{
		Addr:     config.GetRedisAddr(),
		Username: config.GetRedisUsername(),
		Password: config.GetRedisPassword(),
	})
	return &ConversationStorage{
		redisClient: conn,
	}
}

// Get returns the state for the specified conversation key.
// Note that this is checked at each incoming message, so may be a bottleneck for some implementations.
//
// If the key is not found (and as such, this conversation has not yet started), this method should return the
// KeyNotFound error.
func (storage *ConversationStorage) Get(ctx *ext.Context) (*conversation.State, error) {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if state, err := storage.redisClient.Get(context.Background(), key).Result(); err != redis.Nil {
		return &conversation.State{Key: state}, nil
	} else if err != nil {
		return nil, err
	} else {
		return nil, conversation.KeyNotFound
	}
}

// Set updates the conversation state.
func (storage *ConversationStorage) Set(ctx *ext.Context, state conversation.State) error {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if err := storage.redisClient.Set(context.Background(), key, state.Key, time.Hour*24*3).Err(); err != nil {
		return err
	}
	return nil
}

// Delete ends the conversation, removing the key from the storage.
func (storage *ConversationStorage) Delete(ctx *ext.Context) error {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if err := storage.redisClient.Del(context.Background(), key).Err(); err != nil {
		return err
	}
	return nil
}
