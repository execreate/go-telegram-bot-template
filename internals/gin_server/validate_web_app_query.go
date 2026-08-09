package gin_server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// maxWebAppAuthAge bounds how old a WebApp initData payload may be. The HMAC check
// alone accepts a valid payload forever, so we additionally reject stale auth_date
// values to limit the replay window.
const maxWebAppAuthAge = 12 * time.Hour

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
		// Reject payloads whose auth_date is missing, malformed, or too old to
		// guard against replay of a previously captured (still hash-valid) query.
		authDateSec, err := strconv.ParseInt(queryValues.Get("auth_date"), 10, 64)
		if err != nil {
			c.Data(
				http.StatusBadRequest,
				"text/plain; charset=utf-8",
				[]byte("validation failed; invalid auth_date."),
			)
			return
		}
		if age := time.Since(time.Unix(authDateSec, 0)); age > maxWebAppAuthAge {
			c.Data(
				http.StatusUnauthorized,
				"text/plain; charset=utf-8",
				[]byte("validation failed; auth_date expired."),
			)
			return
		}

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
