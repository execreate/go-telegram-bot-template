package helpers

import (
	"context"
	"crypto/tls"
	"time"

	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const defaultExpirationTime = time.Hour * 24 * 3

var cachedStorage *RedisConversationStorage

type RedisConfig interface {
	GetRedisAddr() string
	GetRedisUsername() string
	GetRedisPassword() string
	GetRedisUseSSL() bool
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
	if config.GetRedisUseSSL() {
		redisTlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	logger.Log.Info("connecting to redis", zap.String("redis_addr", config.GetRedisAddr()))
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
		logger.Log.Fatal("failed to ping redis", zap.Error(err))
	} else {
		logger.Log.Info("redis connection success", zap.String("redis_response", res))
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
// ErrKeyNotFound error.
func (storage *RedisConversationStorage) Get(ctx *ext.Context) (*conversation.State, error) {
	key, err := conversation.StateKey(ctx, storage.keyStrategy)
	if err != nil {
		return nil, err
	}
	if state, err := storage.redisClient.Get(context.Background(), key).Result(); err == nil {
		return &conversation.State{Key: state}, nil
	} else if errors.Is(err, redis.Nil) {
		return nil, conversation.ErrKeyNotFound
	} else {
		logger.Log.Error("failed to get key from redis", zap.Error(err))
		return nil, err
	}
}

// Set updates the conversation state.
func (storage *RedisConversationStorage) Set(ctx *ext.Context, state conversation.State) error {
	key, err := conversation.StateKey(ctx, storage.keyStrategy)
	if err != nil {
		return err
	}
	if err := storage.redisClient.Set(context.Background(), key, state.Key, defaultExpirationTime).Err(); err != nil {
		logger.Log.Error("failed to set key to redis", zap.Error(err))
		return err
	}
	return nil
}

// Delete ends the conversation, removing the key from the storage.
func (storage *RedisConversationStorage) Delete(ctx *ext.Context) error {
	key, err := conversation.StateKey(ctx, storage.keyStrategy)
	if err != nil {
		return err
	}
	if err := storage.redisClient.Del(context.Background(), key).Err(); err != nil {
		logger.Log.Error("failed to delete key from redis", zap.Error(err))
		return err
	}
	return nil
}
