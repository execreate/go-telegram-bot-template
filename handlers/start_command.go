package handlers

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/spf13/viper"
)

// Hello replies to a message with a welcome message.
func Hello(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveChat.Type != "private" {
		return nil
	}

	texts := ctx.Data["texts"].(*viper.Viper)

	name := "stranger"
	if ctx.EffectiveUser != nil {
		name = ctx.EffectiveUser.FirstName
	}

	_, err := ctx.EffectiveMessage.Reply(
		b,
		fmt.Sprintf(texts.GetString("hello"), name),
		nil,
	)
	return err
}
