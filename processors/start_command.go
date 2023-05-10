package processors

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"my-telegram-bot/locale"
)

// Hello replies to a message with "hello world".
func Hello(b *gotgbot.Bot, ctx *ext.Context) error {
	texts, err := locale.GetTranslations("en")
	if err != nil {
		return err
	}
	_, err = ctx.EffectiveMessage.Reply(b, texts.GetString("hello"), nil)
	if err != nil {
		return err
	}
	return nil
}
