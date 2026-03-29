package logger

import (
	"log/slog"
	"os"

	slogzap "github.com/samber/slog-zap/v2"
	"go.uber.org/zap"
)

var (
	Log  *zap.Logger
	Slog *slog.Logger
)

func init() {
	var err error

	if os.Getenv("DEBUG") != "" {
		Log, err = zap.NewDevelopment()
	} else {
		Log, err = zap.NewProduction()
	}

	if err != nil {
		panic(err)
	}

	Slog = slog.New(
		slogzap.Option{
			Level:  slog.LevelDebug,
			Logger: Log,
		}.NewZapHandler(),
	)
}

func Flush() {
	_ = Log.Sync()
}
