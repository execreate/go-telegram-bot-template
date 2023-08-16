package locale

import (
	"flag"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"my-telegram-bot/internals/logger"
)

var (
	textLocales   = map[string]*viper.Viper{}
	cmdLocales    = map[string]*viper.Viper{}
	localesConfig *viper.Viper
)

func init() {
	localesConfig = viper.New()
	flag.String("locale-path", "./locale", "path to the folder where locale files are located")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := localesConfig.BindPFlags(pflag.CommandLine)
	if err != nil {
		logger.LogFatal(err, "failed to bind flags")
	}
}

// GetTextTranslations parses the locale file and returns the viper config.
func GetTextTranslations(locale string) (*viper.Viper, error) {
	if locale == "" {
		return GetTextTranslations("en")
	}
	if textLocales[locale] != nil {
		return textLocales[locale], nil
	}
	config := viper.New()
	config.SetConfigName(locale)
	config.SetConfigType("yaml")
	config.AddConfigPath(localesConfig.GetString("locale-path"))
	err := config.ReadInConfig()
	if err != nil {
		logger.LogWarningf(
			"failed to get text translations for locale %v at path %s",
			locale,
			localesConfig.GetString("locale-path"),
		)
		// fallback locale is English
		if locale != "en" {
			return GetTextTranslations("en")
		}
		return nil, err
	}
	textLocales[locale] = config
	return config, nil
}

// GetCmdTranslations parses the locale file and returns the viper config.
func GetCmdTranslations(locale string) (*viper.Viper, error) {
	if locale == "" {
		return GetCmdTranslations("en")
	}
	if cmdLocales[locale] != nil {
		return cmdLocales[locale], nil
	}
	config := viper.New()
	config.SetConfigName(locale + "_commands")
	config.SetConfigType("yaml")
	config.AddConfigPath(localesConfig.GetString("locale-path"))
	err := config.ReadInConfig()
	if err != nil {
		logger.LogWarningf(
			"failed to get command translations for locale %v at path %s",
			locale,
			localesConfig.GetString("locale-path"),
		)
		// fallback locale is English
		if locale != "en" {
			return GetCmdTranslations("en")
		}
		return nil, err
	}
	cmdLocales[locale] = config
	return config, nil
}
