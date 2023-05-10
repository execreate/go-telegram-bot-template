package mylogger

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
}

func LogInfo(msg string) {
	log.Info().Msg(msg)
}

func LogInfof(msg string, args ...interface{}) {
	log.Info().Msgf(msg, args...)
}

func LogWarning(msg string) {
	log.Warn().Msg(msg)
}

func LogWarningf(msg string, args ...interface{}) {
	log.Warn().Msgf(msg, args...)
}

func LogError(err error, msg string) {
	log.Error().Stack().Err(err).Msg(msg)
}

func LogErrorf(err error, msg string, args ...interface{}) {
	log.Error().Stack().Err(err).Msgf(msg, args...)
}

func LogFatal(err error, msg string) {
	log.Fatal().Stack().Err(err).Msg(msg)
}

func LogFatalf(err error, msg string, args ...interface{}) {
	log.Fatal().Stack().Err(err).Msgf(msg, args...)
}
