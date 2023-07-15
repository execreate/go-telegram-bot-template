package static

import (
	"encoding/json"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gin-gonic/gin"
	"my-telegram-bot/mylogger"
	"net/http"
)

type Config interface {
	GetStaticContentPath() string
	GetWebAppPort() int
	GetToken() string
}

type TgWebAppUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

func ServeStaticContent(config Config, termsAccepted func(userID int64)) func() {
	router := gin.Default()
	_ = router.SetTrustedProxies(nil)

	router.GET("/accept_terms", func(c *gin.Context) {
		queryValues := c.Request.URL.Query()

		ok, err := ext.ValidateWebAppQuery(queryValues, config.GetToken())
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
				mylogger.LogError(err, "failed to unmarshal user")
				return
			}
			termsAccepted(webAppUser.ID)

			c.Data(
				http.StatusOK,
				"text/plain; charset=utf-8",
				[]byte("validation success; user is authenticated."),
			)

		} else {
			c.Data(
				http.StatusBadRequest,
				"text/plain; charset=utf-8",
				[]byte("validation failed; data cannot be trusted."),
			)
		}
	})

	router.StaticFile(
		"/terms_and_conditions.html",
		config.GetStaticContentPath()+"/terms_and_conditions.html",
	)

	return func() {
		mylogger.LogInfo("starting static server...")
		err := router.Run(fmt.Sprintf(":%d", config.GetWebAppPort()))
		if err != nil {
			panic(err)
		}
	}
}
