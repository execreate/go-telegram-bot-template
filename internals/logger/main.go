package logger

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"os"
)

var (
	Log zerolog.Logger
)

func init() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	Log = zerolog.New(os.Stdout).With().Timestamp().Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05", NoColor: false})
	Log.Debug().Msg("logger initialized")
}
