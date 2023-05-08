package chat

import (
	"golang.org/x/time/rate"
	"time"
)

type Chat struct {
	ChatId  int64
	limiter *rate.Limiter
}

func NewChat(chatId int64) *Chat {
	return &Chat{
		ChatId:  chatId,
		limiter: rate.NewLimiter(rate.Every(time.Second), 2),
	}
}
