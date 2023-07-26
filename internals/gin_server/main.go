package gin_server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"my-telegram-bot/internals/logger"
)

type Config interface {
	GetStaticContentPath() string
	GetWebAppPort() int
	GetToken() string
}

type Server struct {
	router *gin.Engine
	config Config
}

type HandlerMethods string

const (
	GET  HandlerMethods = "GET"
	POST HandlerMethods = "POST"
)

func NewGinServer(config Config) *Server {
	router := gin.Default()
	_ = router.SetTrustedProxies(nil)

	return &Server{
		router: router,
		config: config,
	}
}

func (srv *Server) RunServer() {
	logger.LogInfo("starting gin server...")
	err := srv.router.Run(fmt.Sprintf(":%d", srv.config.GetWebAppPort()))
	if err != nil {
		panic(err)
	}
}

func (srv *Server) AddWebAppRequestHandler(
	method HandlerMethods,
	path string,
	handlerFn func(c *gin.Context, webAppUser *TgWebAppUser, texts *viper.Viper),
) {
	switch method {
	case GET:
		srv.router.GET(path, func(c *gin.Context) {
			srv.validateWebAppQuery(c, handlerFn)
		})
	case POST:
		srv.router.POST(path, func(c *gin.Context) {
			srv.validateWebAppQuery(c, handlerFn)
		})
	default:
		logger.LogPanic(nil, "unknown handler method")
	}
}

func (srv *Server) AddStaticFileHandler(fileName string) {
	srv.router.StaticFile(
		"/"+fileName,
		srv.config.GetStaticContentPath()+"/"+fileName,
	)
}
