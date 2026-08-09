package logger

import (
	"log/slog"

	slogzap "github.com/samber/slog-zap/v2"
	"go.uber.org/zap"
)

var (
	Log  *zap.Logger
	Slog *slog.Logger
)

func init() {
	// Start with a production logger so that anything logged before configuration
	// is loaded (e.g. reading the config file) still works. Call Configure once the
	// debug flag is known to switch to a development logger when requested.
	build(false)
}

// Configure rebuilds the package loggers according to the debug flag. It must be
// called during startup, before any goroutines use the loggers, since it swaps the
// package-level Log and Slog values.
func Configure(debug bool) {
	build(debug)
}

func build(debug bool) {
	var (
		log *zap.Logger
		err error
	)

	if debug {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}

	if err != nil {
		panic(err)
	}

	Log = log
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
