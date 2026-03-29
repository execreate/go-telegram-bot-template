package gin_server

import (
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type TgWebAppUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	QueryID      string `json:"query_id,omitempty"`
}

func (srv *Server) validateWebAppQuery(c *gin.Context, successCallBack func(*gin.Context, *TgWebAppUser, *viper.Viper)) {
	queryValues := c.Request.URL.Query()
	ok, err := ext.ValidateWebAppQuery(queryValues, srv.config.GetToken())
	if err != nil {
		c.Data(http.StatusBadRequest,
			"text/plain; charset=utf-8",
			[]byte("validation failed; error: "+err.Error()),
		)
		return
	}

	if ok {
		var webAppUser TgWebAppUser
		if err := json.Unmarshal([]byte(queryValues.Get("user")), &webAppUser); err != nil {
			c.Data(
				http.StatusInternalServerError,
				"text/plain; charset=utf-8",
				[]byte("Internal Server Error"),
			)
			logger.Log.Error(
				"failed to unmarshal user",
				zap.Error(err),
			)
			return
		}
		webAppUser.QueryID = queryValues.Get("query_id")

		texts, err := locale.GetTextTranslations(webAppUser.LanguageCode)
		if err != nil {
			logger.Log.Error(
				"failed to get translations",
				zap.Error(err),
			)
			c.Data(
				http.StatusInternalServerError,
				"text/plain; charset=utf-8",
				[]byte("failed to get translation texts"),
			)
			return
		}

		successCallBack(c, &webAppUser, texts)
	} else {
		c.Data(
			http.StatusBadRequest,
			"text/plain; charset=utf-8",
			[]byte("validation failed; data cannot be trusted."),
		)
	}
}
