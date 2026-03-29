package handlers

import (
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/bot"
	"github.com/execreate/go-telegram-bot-template/internals/gin_server"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type TermsAndConditionsHandler struct {
	bot      *bot.MyBot
	htmlFile string
	version  string
}

func NewTermsAndConditionsHandler(bot *bot.MyBot, srv *gin_server.Server) *TermsAndConditionsHandler {
	termsHandler := &TermsAndConditionsHandler{
		bot:      bot,
		htmlFile: "terms_and_conditions.html",
		version:  "v1.0.0",
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
	return ctx.EffectiveChat != nil && ctx.EffectiveChat.Type == "private" && user.MustAcceptTermsAndConditions(handler.version)
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
	if user.AcceptedTermsAndConditionsOn.Valid && user.MustAcceptTermsAndConditions(handler.version) {
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
	if err := handler.bot.UsersCache.UserHasAcceptedTermsAndConditions(webAppUser.ID, handler.version); err != nil {
		logger.Log.Error(
			"failed to update user's terms and conditions acceptance status",
			zap.Int64("user_id", webAppUser.ID),
			zap.Error(err),
		)
		_, err = handler.bot.SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.failed_to_accept"), nil)
		if err != nil {
			logger.Log.Error(
				"failed to send message to user",
				zap.Int64("user_id", webAppUser.ID),
				zap.Error(err),
			)
		}
	} else {
		_, err := handler.bot.SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.accepted"), nil)
		if err != nil {
			logger.Log.Error(
				"failed to send message to user",
				zap.Int64("user_id", webAppUser.ID),
				zap.Error(err),
			)
		}
	}

	c.Data(
		http.StatusOK,
		"text/plain; charset=utf-8",
		[]byte("success"),
	)
}
