package configuration

import "testing"

func TestConfigureDefaults(t *testing.T) {
	// No config.yaml exists in the package directory, so every value here comes from
	// the defaults set in Configure.
	config := Configure(nil)

	if got := config.GetWebhookListenAddr(); got != "0.0.0.0" {
		t.Errorf("GetWebhookListenAddr() = %q, want %q", got, "0.0.0.0")
	}
	if got := config.GetWebhookPort(); got != 8080 {
		t.Errorf("GetWebhookPort() = %d, want 8080", got)
	}
	if got := config.GetWebAppPort(); got != 8081 {
		t.Errorf("GetWebAppPort() = %d, want 8081", got)
	}
	if got := config.GetTermsAndConditionsVersion(); got != "v1.0.0" {
		t.Errorf("GetTermsAndConditionsVersion() = %q, want %q", got, "v1.0.0")
	}
	if config.GetRedisUseSSL() {
		t.Error("GetRedisUseSSL() = true, want false when unset")
	}
	if config.GetDebug() {
		t.Error("GetDebug() = true, want false when unset")
	}
}

func TestGetRedisUseSSL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "1", value: "1", want: true},
		// The former implementation treated any non-empty value as true, so "false"
		// silently turned TLS on.
		{name: "false", value: "false", want: false},
		{name: "0", value: "0", want: false},
		{name: "empty", value: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MY_BOT_REDIS_USE_SSL", tt.value)

			if got := Configure(nil).GetRedisUseSSL(); got != tt.want {
				t.Errorf("GetRedisUseSSL() with MY_BOT_REDIS_USE_SSL=%q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestEnvOverridesAndTrimming(t *testing.T) {
	t.Setenv("MY_BOT_WEBHOOK_LISTEN_ADDR", "127.0.0.1")
	t.Setenv("MY_BOT_WEBHOOK_DOMAIN", "https://bot.example.com/")
	t.Setenv("MY_BOT_WEBAPP_DOMAIN", "https://app.example.com/")
	t.Setenv("MY_BOT_TOKEN", "123:ABC")

	config := Configure([]string{"token"})

	if got := config.GetWebhookListenAddr(); got != "127.0.0.1" {
		t.Errorf("GetWebhookListenAddr() = %q, want %q", got, "127.0.0.1")
	}
	if got := config.GetWebhookDomain(); got != "https://bot.example.com" {
		t.Errorf("GetWebhookDomain() = %q, want the trailing slash trimmed", got)
	}
	if got := config.GetWebAppDomain(); got != "https://app.example.com" {
		t.Errorf("GetWebAppDomain() = %q, want the trailing slash trimmed", got)
	}
	if got := config.GetWebhookPath(); got != "123:ABC/webhook" {
		t.Errorf("GetWebhookPath() = %q, want %q", got, "123:ABC/webhook")
	}
}
