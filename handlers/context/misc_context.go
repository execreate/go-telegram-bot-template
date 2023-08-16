package context

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"my-telegram-bot/locale"
)

type MiscContextHandler struct {
	webAppDomain string
}

func NewMiscContextHandler(webAppDomain string) *MiscContextHandler {
	return &MiscContextHandler{webAppDomain}
}

func (miscCtx MiscContextHandler) CheckUpdate(_ *gotgbot.Bot, _ *ext.Context) bool {
	return true
}

func (miscCtx MiscContextHandler) HandleUpdate(_ *gotgbot.Bot, ctx *ext.Context) error {
	ctx.Data["webapp_domain"] = miscCtx.webAppDomain

	langCode := "en"
	if ctx.EffectiveUser != nil {
		langCode = ctx.EffectiveUser.LanguageCode
	}

	texts, err := locale.GetTextTranslations(langCode)
	if err != nil {
		return err
	}
	ctx.Data["texts"] = texts

	return nil
}

func (miscCtx MiscContextHandler) Name() string {
	return "MiscContextHandler"
}
