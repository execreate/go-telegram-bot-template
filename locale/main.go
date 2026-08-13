package locale

import (
	"flag"
	"sync"

	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	textLocales   = map[string]*viper.Viper{}
	textLocalesMu sync.RWMutex
	cmdLocales    = map[string]*viper.Viper{}
	cmdLocalesMu  sync.RWMutex
	localesConfig *viper.Viper
)

const FallbackLanguage = "en"

func init() {
	localesConfig = viper.New()
	flag.String("locale-path", "./locale", "path to the folder where locale files are located")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Binding before the flags are parsed is fine: viper reads through to the flag, so
	// it picks up whatever ParseFlags later sets, and falls back to the default until
	// then. Parsing here instead would consume os.Args on import — which breaks any
	// binary that owns its own flags, and swallows `go test`'s own flags (notably
	// -test.coverprofile, leaving every importing package with an empty profile).
	if err := localesConfig.BindPFlags(pflag.CommandLine); err != nil {
		logger.Log.Fatal("failed to bind flags", zap.Error(err))
	}
}

// ParseFlags parses the command line so --locale-path takes effect. Call it once from
// main, before anything resolves a translation. Binaries that manage their own flags
// can skip it and call SetPath instead.
func ParseFlags() {
	pflag.Parse()
}

// SetPath overrides the directory locale files are read from and drops the parsed
// locales already cached under the previous path. The --locale-path flag remains the
// default source; this exists for tests and for embedding the template in a binary
// that resolves its own paths.
func SetPath(path string) {
	textLocalesMu.Lock()
	textLocales = map[string]*viper.Viper{}
	textLocalesMu.Unlock()

	cmdLocalesMu.Lock()
	cmdLocales = map[string]*viper.Viper{}
	cmdLocalesMu.Unlock()

	localesConfig.Set("locale-path", path)
}

// GetTextTranslations parses the locale file and returns the viper config.
func GetTextTranslations(locale string) (*viper.Viper, error) {
	if locale == "" {
		return GetTextTranslations(FallbackLanguage)
	}

	textLocalesMu.RLock()
	cached := textLocales[locale]
	textLocalesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	config := viper.New()
	config.SetConfigName(locale)
	config.SetConfigType("yaml")
	config.AddConfigPath(localesConfig.GetString("locale-path"))
	err := config.ReadInConfig()
	if err != nil {
		logger.Log.Warn(
			"failed to get text translations for locale",
			zap.String("locale", locale),
			zap.String("locale_path", localesConfig.GetString("locale-path")),
			zap.Error(err),
		)
		if locale != FallbackLanguage {
			return GetTextTranslations(FallbackLanguage)
		}
		return nil, err
	}

	textLocalesMu.Lock()
	textLocales[locale] = config
	textLocalesMu.Unlock()
	return config, nil
}

// GetCmdTranslations parses the locale file and returns the viper config.
func GetCmdTranslations(locale string) (*viper.Viper, error) {
	if locale == "" {
		return GetCmdTranslations(FallbackLanguage)
	}

	cmdLocalesMu.RLock()
	cached := cmdLocales[locale]
	cmdLocalesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	config := viper.New()
	config.SetConfigName(locale + "_commands")
	config.SetConfigType("yaml")
	config.AddConfigPath(localesConfig.GetString("locale-path"))
	err := config.ReadInConfig()
	if err != nil {
		logger.Log.Warn(
			"failed to get command translations for locale",
			zap.String("locale", locale),
			zap.String("locale_path", localesConfig.GetString("locale-path")),
			zap.Error(err),
		)
		if locale != FallbackLanguage {
			return GetCmdTranslations(FallbackLanguage)
		}
		return nil, err
	}

	cmdLocalesMu.Lock()
	cmdLocales[locale] = config
	cmdLocalesMu.Unlock()
	return config, nil
}
