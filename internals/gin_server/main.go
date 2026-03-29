package gin_server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
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

func (srv *Server) GetServer(ctx context.Context) *http.Server {
	return &http.Server{
		Addr: fmt.Sprintf(":%d", srv.config.GetWebAppPort()),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: srv.router.Handler(),
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
		logger.Log.Panic("unknown handler method")
	}
}

func (srv *Server) AddStaticFileHandler(fileName string) {
	srv.router.StaticFile(
		"/"+fileName,
		srv.config.GetStaticContentPath()+"/"+fileName,
	)
}

func (srv *Server) AddStaticFolderHandler(urlPath, folderPath string) {
	srv.router.Static(urlPath, srv.config.GetStaticContentPath()+"/"+folderPath)
}
