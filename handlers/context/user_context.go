package context

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"my-telegram-bot/internals/users_cache"
)

type UserContextHandler struct {
	users *users_cache.TgUsersCache
}

func NewUserContextHandler(usersCache *users_cache.TgUsersCache) *UserContextHandler {
	return &UserContextHandler{
		users: usersCache,
	}
}

func (usrCtx *UserContextHandler) CheckUpdate(_ *gotgbot.Bot, ctx *ext.Context) bool {
	user := ctx.EffectiveUser
	return user != nil
}

func (usrCtx *UserContextHandler) HandleUpdate(_ *gotgbot.Bot, ctx *ext.Context) error {
	if dbUser, err := usrCtx.users.Get(ctx.EffectiveUser); err != nil {
		return err
	} else {
		ctx.Data["db_user"] = dbUser
	}
	return nil
}

func (usrCtx *UserContextHandler) Name() string {
	return "UserContextHandler"
}
