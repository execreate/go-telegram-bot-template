package configuration

import (
	"github.com/spf13/viper"
	"my-telegram-bot/internals/logger"
	"os"
	"strings"
)

// Configuration keeps bot configuration settings
type Configuration struct {
	*viper.Viper
}

func Configure() *Configuration {
	// configure viper
	config := &Configuration{viper.New()}
	config.SetEnvPrefix("my_bot")  // will be upper-cased automatically
	config.AutomaticEnv()          // automatically read in environment variables that match
	config.AddConfigPath(".")      // optionally look for config in the working directory
	config.SetConfigName("config") // name of config file (without extension)
	config.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	err := config.ReadInConfig()   // Find and read the config file
	if err != nil {                // Handle errors reading the config file
		logger.LogInfof("error reading the config file (%v), fallback to env variables", err)
	}
	config.SetDefault("webhook_port", 8080)
	config.SetDefault("webapp_port", 8081)
	checkRequiredEnvVariables(config)
	return config
}

func checkRequiredEnvVariables(config *Configuration) {
	// Check token is set in the environment variable.
	token := config.GetString("token")
	if token == "" {
		logger.LogFatal(nil, "TOKEN configuration variable is empty")
	}

	// Check the webhook domain is set in the environment variable.
	webhookDomain := config.GetString("webhook_domain")
	if webhookDomain == "" {
		logger.LogFatal(nil, "WEBHOOK_DOMAIN configuration variable is empty")
	}

	// Check the webhook domain is set in the environment variable.
	webAppDomain := config.GetString("webapp_domain")
	if webAppDomain == "" {
		logger.LogFatal(nil, "WEBAPP_DOMAIN configuration variable is empty")
	}

	// Check the webhook secret is set in the environment variable.
	webhookSecret := config.GetString("webhook_secret")
	if webhookSecret == "" {
		logger.LogFatal(nil, "WEBHOOK_SECRET configuration variable is empty")
	}

	// Check the static content path is set in the environment variable.
	staticContentPath := config.GetString("static_content_path")
	if staticContentPath == "" {
		logger.LogFatal(nil, "STATIC_CONTENT_PATH configuration variable is empty")
	}
}

// GetToken returns bots secret token
func (config *Configuration) GetToken() string {
	return config.GetString("token")
}

// GetWebhookDomain returns the webhook domain without the trailing slash
func (config *Configuration) GetWebhookDomain() string {
	webhookDomainWithoutTrailingSlash, _ := strings.CutSuffix(config.GetString("webhook_domain"), "/")
	return webhookDomainWithoutTrailingSlash
}

// GetWebhookPath returns the webhook path
func (config *Configuration) GetWebhookPath() string {
	webhookPath := "webhook_" + strings.Split(config.GetToken(), ":")[0]
	return webhookPath
}

// GetWebhookPort returns the webhook port
func (config *Configuration) GetWebhookPort() int {
	return config.GetInt("webhook_port")
}

// GetWebAppDomain returns the WebApp domain without the trailing slash
func (config *Configuration) GetWebAppDomain() string {
	webAppDomainWithoutTrailingSlash, _ := strings.CutSuffix(config.GetString("webapp_domain"), "/")
	return webAppDomainWithoutTrailingSlash
}

// GetWebAppPort returns the webhook port
func (config *Configuration) GetWebAppPort() int {
	return config.GetInt("webapp_port")
}

// GetWebhookSecret returns webhook secret
func (config *Configuration) GetWebhookSecret() string {
	return config.GetString("webhook_secret")
}

// GetStaticContentPath returns path to the static content without the trailing path separator
func (config *Configuration) GetStaticContentPath() string {
	pathWithoutTrailingSlash, _ := strings.CutSuffix(config.GetString("static_content_path"), string(os.PathSeparator))
	return pathWithoutTrailingSlash
}

// GetDbDSN returns the database connection string
func (config *Configuration) GetDbDSN() string {
	return config.GetString("db_dsn")
}
