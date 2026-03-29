package configuration

import (
	"os"
	"strings"

	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Configuration keeps bot configuration settings
type Configuration struct {
	*viper.Viper
}

func Configure(requiredConfigVariables []string) *Configuration {
	// configure viper
	config := &Configuration{viper.New()}
	config.SetEnvPrefix("my_bot")  // will be upper-cased automatically
	config.AutomaticEnv()          // automatically read in environment variables that match
	config.AddConfigPath(".")      // optionally look for config in the working directory
	config.SetConfigName("config") // name of config file (without extension)
	config.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	err := config.ReadInConfig()   // Find and read the config file
	if err != nil {                // Handle errors reading the config file
		logger.Log.Info(
			"error reading the configuration file fallback to env variables",
			zap.Error(err),
		)
	}
	config.SetDefault("webhook_port", 8080)
	config.SetDefault("webapp_port", 8081)
	checkRequiredConfigVariables(config, requiredConfigVariables)
	return config
}

func checkRequiredConfigVariables(config *Configuration, requiredConfigVariables []string) {
	for _, variableName := range requiredConfigVariables {
		// Check the config variable is set.
		if config.GetString(variableName) == "" {
			logger.Log.Fatal(
				"required configuration variable is empty",
				zap.String("config_var_name", variableName),
			)
		}
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
	webhookPath := config.GetToken() + "/webhook"
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

// GetRedisAddr returns the database connection string
func (config *Configuration) GetRedisAddr() string {
	return config.GetString("redis_addr")
}

// GetRedisUsername returns the database connection string
func (config *Configuration) GetRedisUsername() string {
	return config.GetString("redis_user")
}

// GetRedisPassword returns the database connection string
func (config *Configuration) GetRedisPassword() string {
	return config.GetString("redis_pass")
}

func (config *Configuration) GetRedisUseSSL() bool {
	return config.GetString("redis_use_ssl") != ""
}

// GetDebug returns true if the app is running in debug mode
func (config *Configuration) GetDebug() bool {
	return config.GetString("debug") != "" || os.Getenv("DEBUG") != ""
}
