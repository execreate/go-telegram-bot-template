package helpers

import (
	"context"
	"crypto/tls"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"my-telegram-bot/internals/logger"
	"time"
)

const defaultExpirationTime = time.Hour * 24 * 3

var cachedStorage *RedisConversationStorage

type RedisConfig interface {
	GetRedisAddr() string
	GetRedisUsername() string
	GetRedisPassword() string
	GetEnvironment() string
}

type RedisConversationStorage struct {
	// keyStrategy defines how to calculate keys for each conversation.
	keyStrategy conversation.KeyStrategy
	// redisClient keeps redis connection
	redisClient *redis.Client
}

func NewRedisConversationStorage(config RedisConfig, botUsername string) *RedisConversationStorage {
	if cachedStorage != nil {
		return cachedStorage
	}

	var redisTlsConfig *tls.Config
	if config.GetEnvironment() == "production" {
		redisTlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	logger.Log.Info().Str("redis_addr", config.GetRedisAddr()).Msg("connecting to redis")
	conn := redis.NewClient(&redis.Options{
		Addr:       config.GetRedisAddr(),
		Username:   config.GetRedisUsername(),
		Password:   config.GetRedisPassword(),
		ClientName: "telegram-bot:" + botUsername,
		TLSConfig:  redisTlsConfig,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if res, err := conn.Ping(ctx).Result(); err != nil {
		logger.Log.Fatal().Err(err).Msg("failed to ping redis")
	} else {
		logger.Log.Info().Str("response", res).Msg("redis connection success")
	}

	cachedStorage = &RedisConversationStorage{
		redisClient: conn,
	}

	return cachedStorage
}

// Get returns the state for the specified conversation key.
// Note that this is checked at each incoming message, so may be a bottleneck for some implementations.
//
// If the key is not found (and as such, this conversation has not yet started), this method should return the
// KeyNotFound error.
func (storage *RedisConversationStorage) Get(ctx *ext.Context) (*conversation.State, error) {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if state, err := storage.redisClient.Get(context.Background(), key).Result(); err == nil {
		return &conversation.State{Key: state}, nil
	} else if errors.Is(err, redis.Nil) {
		return nil, conversation.KeyNotFound
	} else {
		logger.Log.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to get key from redis")
		return nil, err
	}
}

// Set updates the conversation state.
func (storage *RedisConversationStorage) Set(ctx *ext.Context, state conversation.State) error {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if err := storage.redisClient.Set(context.Background(), key, state.Key, defaultExpirationTime).Err(); err != nil {
		logger.Log.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to set key to redis")
		return err
	}
	return nil
}

// Delete ends the conversation, removing the key from the storage.
func (storage *RedisConversationStorage) Delete(ctx *ext.Context) error {
	key := conversation.StateKey(ctx, storage.keyStrategy)
	if err := storage.redisClient.Del(context.Background(), key).Err(); err != nil {
		logger.Log.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msg("failed to delete key from redis")
		return err
	}
	return nil
}
