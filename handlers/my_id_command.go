package handlers

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// MyID replies to a command by sending user ID.
func MyID(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(
		b,
		fmt.Sprintf("ID: %d", ctx.EffectiveUser.Id),
		nil,
	)
	return err
}
