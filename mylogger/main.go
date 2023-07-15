package mylogger

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"os"
)

var (
	logger zerolog.Logger
)

func init() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	logger = zerolog.New(os.Stdout).With().Timestamp().Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05", NoColor: false})
	logger.Debug().Msg("logger initialized")
}

func LogInfo(msg string) {
	logger.Info().Msg(msg)
}

func LogInfof(msg string, args ...interface{}) {
	logger.Info().Msgf(msg, args...)
}

func LogWarning(msg string) {
	logger.Warn().Msg(msg)
}

func LogWarningf(msg string, args ...interface{}) {
	logger.Warn().Msgf(msg, args...)
}

func LogError(err error, msg string) {
	logger.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msg(msg)
}

func LogErrorf(err error, msg string, args ...interface{}) {
	logger.Error().Stack().Err(errors.Wrap(err, "wrapped error")).Msgf(msg, args...)
}

func LogFatal(err error, msg string) {
	logger.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msg(msg)
}

func LogFatalf(err error, msg string, args ...interface{}) {
	logger.Fatal().Stack().Err(errors.Wrap(err, "wrapped error")).Msgf(msg, args...)
}

func LogPanic(reason any, msg string) {
	logger.Panic().Any("panic_reason", reason).Msg(msg)
}
