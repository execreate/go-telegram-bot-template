package locale

import (
	"flag"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"my-telegram-bot/internals/mylogger"
)

var (
	locales       = map[string]*viper.Viper{}
	localesConfig *viper.Viper
)

func init() {
	localesConfig = viper.New()
	flag.String("locale-path", "./locale", "path to the folder where locale files are located")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := localesConfig.BindPFlags(pflag.CommandLine)
	if err != nil {
		mylogger.LogFatal(err, "failed to bind flags")
	}
}

// GetTranslations parses the locale file and returns the viper config.
func GetTranslations(locale string) (*viper.Viper, error) {
	if locales[locale] != nil {
		return locales[locale], nil
	}
	config := viper.New()
	config.SetConfigName(locale)
	config.SetConfigType("yaml")
	config.AddConfigPath(localesConfig.GetString("locale_path"))
	err := config.ReadInConfig()
	if err != nil {
		mylogger.LogErrorf(err, "failed to get translations for locale %v", locale)
		return nil, err
	}
	locales[locale] = config
	return config, nil
}
