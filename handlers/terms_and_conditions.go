package handlers

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/bot"
	"github.com/execreate/go-telegram-bot-template/internals/gin_server"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type TermsAndConditionsHandler struct {
	bot                *bot.MyBot
	htmlFilename       string
	version            string
	supportedLanguages []string
}

func NewTermsAndConditionsHandler(
	bot *bot.MyBot,
	srv *gin_server.Server,
	supportedLanguages []string,
	version string,
) (*TermsAndConditionsHandler, error) {
	termsHandler := &TermsAndConditionsHandler{
		bot:                bot,
		htmlFilename:       "terms_and_conditions",
		version:            version,
		supportedLanguages: supportedLanguages,
	}

	for _, lang := range supportedLanguages {
		srv.AddStaticFileHandler(
			fmt.Sprintf("%s.%s.html", termsHandler.htmlFilename, lang),
		)
	}

	if err := srv.AddWebAppRequestHandler(
		gin_server.GET,
		"/accept_terms",
		termsHandler.handleAcceptTermsAndConditions,
	); err != nil {
		return nil, fmt.Errorf("failed to register accept_terms handler: %w", err)
	}

	return termsHandler, nil
}

func (handler *TermsAndConditionsHandler) CheckUpdate(_ *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveUser == nil {
		return false
	}
	// UserContextHandler at group -1 populates db_user, but a fork that registers a
	// handler at group -1 or lower, or reorders the groups, may reach this first. Skip
	// the update instead of panicking in the dispatcher.
	user, ok := ctx.Data["db_user"].(*tables.TelegramUser)
	if !ok {
		return false
	}
	return ctx.EffectiveChat != nil && ctx.EffectiveChat.Type == "private" && user.MustAcceptTermsAndConditions(handler.version)
}

func (handler *TermsAndConditionsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	texts := ctx.Data["texts"].(*viper.Viper)
	user := ctx.Data["db_user"].(*tables.TelegramUser)

	pageLanguage := locale.FallbackLanguage
	if slices.Contains(handler.supportedLanguages, user.LanguageCode) {
		pageLanguage = user.LanguageCode
	}

	pageURL := fmt.Sprintf(
		"%s/%s.%s.html",
		ctx.Data["webapp_domain"],
		handler.htmlFilename,
		pageLanguage,
	)

	opts := &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: "Terms and Conditions",
						WebApp: &gotgbot.WebAppInfo{
							Url: pageURL,
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
	if err := handler.bot.UsersCache.UserHasAcceptedTermsAndConditions(
		webAppUser.ID,
		handler.version,
	); err != nil {
		logger.Log.Error(
			"failed to update user's terms and conditions acceptance status",
			zap.Int64("user_id", webAppUser.ID),
			zap.Error(err),
		)
		_, err = handler.bot.Bot().SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.failed_to_accept"), nil)
		if err != nil {
			logger.Log.Error(
				"failed to send message to user",
				zap.Int64("user_id", webAppUser.ID),
				zap.Error(err),
			)
		}
	} else {
		_, err := handler.bot.Bot().SendMessage(webAppUser.ID, texts.GetString("terms_and_conditions.accepted"), nil)
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
		[]byte("OK"),
	)
}
