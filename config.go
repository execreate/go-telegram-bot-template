package main

import (
	"github.com/spf13/viper"
	"my-telegram-bot/mylogger"
)

type Configuration struct {
	*viper.Viper
}

func configure() *Configuration {
	// configure viper
	config := &Configuration{viper.New()}
	config.SetEnvPrefix("my_bot")  // will be upper-cased automatically
	config.AutomaticEnv()          // automatically read in environment variables that match
	config.AddConfigPath(".")      // optionally look for config in the working directory
	config.SetConfigName("config") // name of config file (without extension)
	config.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	err := config.ReadInConfig()   // Find and read the config file
	if err != nil {                // Handle errors reading the config file
		mylogger.LogInfof("error reading the config file (%v), fallback to env variables", err)
	}
	config.SetDefault("listen_port", 8080)
	checkRequiredEnvVariables(config)
	return config
}

func checkRequiredEnvVariables(config *Configuration) {
	// Get token from the environment variable.
	token := config.GetString("token")
	if token == "" {
		mylogger.LogFatal(nil, "TOKEN configuration variable is empty")
	}

	// Get the webhook domain from the environment variable.
	webhookDomain := config.GetString("webhook_domain")
	if webhookDomain == "" {
		mylogger.LogFatal(nil, "WEBHOOK_DOMAIN configuration variable is empty")
	}

	// Get the webhook secret from the environment variable.
	webhookSecret := config.GetString("webhook_secret")
	if webhookSecret == "" {
		mylogger.LogFatal(nil, "WEBHOOK_SECRET configuration variable is empty")
	}
}

func (config *Configuration) GetToken() string {
	return config.GetString("token")
}
func (config *Configuration) GetWebhookDomain() string {
	return config.GetString("webhook_domain")
}
func (config *Configuration) GetWebhookPort() int {
	return config.GetInt("webhook_port")
}
func (config *Configuration) GetWebhookSecret() string {
	return config.GetString("webhook_secret")
}
