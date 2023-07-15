package handlers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/spf13/viper"
	"my-telegram-bot/database/tables"
)

type TermsAndConditionsHandler struct{}

func NewTermsAndConditionsHandler() *TermsAndConditionsHandler {
	return &TermsAndConditionsHandler{}
}

func (terms TermsAndConditionsHandler) CheckUpdate(_ *gotgbot.Bot, ctx *ext.Context) bool {
	user := ctx.Data["db_user"].(*tables.TelegramUser)
	return !user.HasAcceptedTermsAndConditions || !user.HasAcceptedLatestTermsAndConditions
}

func (terms TermsAndConditionsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	texts := ctx.Data["texts"].(*viper.Viper)
	user := ctx.Data["db_user"].(*tables.TelegramUser)

	opts := &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: "Terms and Conditions",
						WebApp: &gotgbot.WebAppInfo{
							Url: ctx.Data["webapp_domain"].(string) + "/terms_and_conditions.html",
						},
					},
				},
			},
		},
	}

	replyMsgText := texts.GetString("terms_and_conditions")
	if user.HasAcceptedTermsAndConditions && !user.HasAcceptedLatestTermsAndConditions {
		replyMsgText = texts.GetString("terms_and_conditions_changed")
	}

	_, err := ctx.EffectiveMessage.Reply(b, replyMsgText, opts)

	if err != nil {
		return err
	}

	return ext.EndGroups
}

func (terms TermsAndConditionsHandler) Name() string {
	return "TermsAndConditionsHandler"
}
