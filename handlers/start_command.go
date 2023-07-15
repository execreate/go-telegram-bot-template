package handlers

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/spf13/viper"
)

// Hello replies to a message with a welcome message.
func Hello(b *gotgbot.Bot, ctx *ext.Context) error {
	texts := ctx.Data["texts"].(*viper.Viper)
	_, err := ctx.EffectiveMessage.Reply(
		b,
		fmt.Sprintf(texts.GetString("hello"), ctx.EffectiveUser.FirstName),
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}
