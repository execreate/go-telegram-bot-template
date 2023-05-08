package main

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func configure() {
	viper.SetEnvPrefix("my_bot")  // will be upper-cased automatically
	viper.AutomaticEnv()          // automatically read in environment variables that match
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
		log.Warn().Msgf("error reading the config file: %w \n will attempt to read config from env", err)
	}
	checkRequiredEnvVariables()
}

func checkRequiredEnvVariables() {
	// Get token from the environment variable.
	token := viper.GetString("token")
	if token == "" {
		log.Fatal().Msg("TOKEN configuration variable is empty")
	}

	// Get the webhook domain from the environment variable.
	webhookDomain := viper.GetString("webhook_domain")
	if webhookDomain == "" {
		log.Fatal().Msg("WEBHOOK_DOMAIN configuration variable is empty")
	}

	// Get the webhook secret from the environment variable.
	webhookSecret := viper.GetString("webhook_secret")
	if webhookSecret == "" {
		log.Fatal().Msg("WEBHOOK_SECRET configuration variable is empty")
	}
}
