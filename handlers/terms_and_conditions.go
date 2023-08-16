package handlers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"my-telegram-bot/database/tables"
	"my-telegram-bot/internals/bot"
	"my-telegram-bot/internals/gin_server"
	"my-telegram-bot/internals/logger"
	"net/http"
)

type TermsAndConditionsHandler struct {
	bot      *bot.MyBot
	htmlFile string
}

func NewTermsAndConditionsHandler(bot *bot.MyBot, srv *gin_server.Server) *TermsAndConditionsHandler {
	termsHandler := &TermsAndConditionsHandler{
		bot:      bot,
		htmlFile: "terms_and_conditions.html",
	}

	srv.AddStaticFileHandler(termsHandler.htmlFile)

	srv.AddWebAppRequestHandler(
		gin_server.GET,
		"/accept_terms",
		termsHandler.handleAcceptTermsAndConditions,
	)

	return termsHandler
}

func (handler *TermsAndConditionsHandler) CheckUpdate(_ *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveUser == nil {
		return false
	}
	user := ctx.Data["db_user"].(*tables.TelegramUser)
	return ctx.EffectiveChat != nil && ctx.EffectiveChat.Type == "private" &&
		(!user.AcceptedLatestTermsAndConditions ||
			user.AcceptedTermsAndConditionsOn == nil || user.AcceptedTermsAndConditionsOn.IsZero())
}

func (handler *TermsAndConditionsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	texts := ctx.Data["texts"].(*viper.Viper)
	user := ctx.Data["db_user"].(*tables.TelegramUser)

	opts := &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: "Terms and Conditions",
						WebApp: &gotgbot.WebAppInfo{
							Url: ctx.Data["webapp_domain"].(string) + "/" + handler.htmlFile,
						},
					},
				},
			},
		},
	}

	replyMsgText := texts.GetString("terms_and_conditions.request")
	if user.AcceptedTermsAndConditionsOn != nil &&
		!user.AcceptedTermsAndConditionsOn.IsZero() &&
		!user.AcceptedLatestTermsAndConditions {
		replyMsgText = texts.GetString("terms_and_conditions.changed")
	}

	_, err := ctx.EffectiveMessage.Reply(b, replyMsgText, opts)

	if err != nil {
		return err
	}

	return ext.EndGroups
}

func (handler *TermsAndConditionsHandler) Name() string {
	return "TermsAndConditionsHandler"
}

func (handler *TermsAndConditionsHandler) handleAcceptTermsAndConditions(
	c *gin.Context,
	webAppUser *gin_server.TgWebAppUser,
	texts *viper.Viper,
) {
	if err := handler.bot.UsersCache.UserHasAcceptedTermsAndConditions(webAppUser.ID); err != nil {
		logger.LogError(err, "failed to update user's terms and conditions acceptance status")
		_, err = handler.bot.SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.failed_to_accept"), nil)
		if err != nil {
			logger.LogError(err, "failed to send message to user")
		}
	} else {
		_, err := handler.bot.SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.accepted"), nil)
		if err != nil {
			logger.LogError(err, "failed to send message to user")
		}
	}

	c.Data(
		http.StatusOK,
		"text/plain; charset=utf-8",
		[]byte("validation success, user is authenticated"),
	)
}
