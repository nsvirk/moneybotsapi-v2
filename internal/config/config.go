// Package config loads configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
)

// Config represents the application configuration
type Config struct {
	APIName              string `env:"MB_API_APP_NAME"`
	APIVersion           string `env:"MB_API_APP_VERSION"`
	ServerPort           string `env:"MB_API_SERVER_PORT"`
	ServerLogLevel       string `env:"MB_API_SERVER_LOG_LEVEL"`
	PostgresDsn          string `env:"MB_API_PG_DSN"`
	PostgresSchema       string `env:"MB_API_PG_SCHEMA"`
	PostgresLogLevel     string `env:"MB_API_PG_LOG_LEVEL"`
	RedisHost            string `env:"MB_API_REDIS_HOST"`
	RedisPort            string `env:"MB_API_REDIS_PORT"`
	RedisPassword        string `env:"MB_API_REDIS_PASSWORD"`
	TelegramBotToken     string `env:"MB_API_TELEGRAM_BOT_TOKEN"`
	TelegramChatID       string `env:"MB_API_TELEGRAM_CHAT_ID"`
	KitetickerUserID     string `env:"MB_API_KITETICKER_USER_ID"`
	KitetickerPassword   string `env:"MB_API_KITETICKER_PASSWORD"`
	KitetickerTotpSecret string `env:"MB_API_KITETICKER_TOTP_SECRET"`
}

var (
	SingleLine string = "--------------------------------------------------"
)

var (
	instance *Config
	once     sync.Once
	err      error
)

// Get returns the application configuration
func Get() (*Config, error) {
	zaplogger.Info(SingleLine)
	zaplogger.Info("Loading Configuration")

	once.Do(func() {
		instance, err = loadConfig()
	})
	return instance, err
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	cfg := &Config{}
	if err := cfg.loadFromEnv(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	t := reflect.TypeOf(*c)
	v := reflect.ValueOf(c).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			return fmt.Errorf("missing env tag for field %s", field.Name)
		}

		value := os.Getenv(envTag)
		if value == "" {
			return fmt.Errorf("env variable %s is required but not set", envTag)
		}

		v.Field(i).SetString(value)
	}

	return nil
}

// String returns the configuration as a string
func (c *Config) String() string {
	var sb strings.Builder
	sb.WriteString("\n--------------------------------------\n")
	sb.WriteString("Configuration:\n")
	sb.WriteString("--------------------------------------\n")

	t := reflect.TypeOf(*c)
	v := reflect.ValueOf(*c)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i).String()

		// Mask sensitive fields
		value = maskSensitiveField(field.Name, value)
		sb.WriteString(fmt.Sprintf("  %s:  %s\n", field.Name, value))
	}

	sb.WriteString("--------------------------------------\n")

	return sb.String()
}

func maskSensitiveField(fieldName, value string) string {
	sensitiveFields := []string{"token", "dsn", "secret", "password", "url"}

	fieldNameLower := strings.ToLower(fieldName)
	for _, sensitive := range sensitiveFields {
		if strings.Contains(fieldNameLower, sensitive) {
			return maskValue(value)
		}
	}

	return value
}

func maskValue(value string) string {
	if len(value) <= 3 {
		return strings.Repeat("*", 7)
	}
	return value[:3] + strings.Repeat("*", 7)
}
