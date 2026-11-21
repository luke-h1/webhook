package webhook

import (
	"errors"
	"os"
	"strings"

	"github.com/luke-h1/webhook/internal/notify"
)

type Config struct {
	AppeekyAPIKey      string
	SentryClientSecret string
	Environment        string
	Notify             notify.Config
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		AppeekyAPIKey:      strings.TrimSpace(os.Getenv("APPEEKY_API_KEY")),
		SentryClientSecret: strings.TrimSpace(os.Getenv("SENTRY_CLIENT_SECRET")),
		Environment:        strings.TrimSpace(os.Getenv("ENVIRONMENT")),
		Notify: notify.Config{
			DiscordWebhookURL: strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
			TelegramBotToken:  strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
			TelegramChatID:    strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		},
	}

	if cfg.AppeekyAPIKey == "" && cfg.SentryClientSecret == "" {
		return nil, errors.New("at least one of APPEEKY_API_KEY or SENTRY_CLIENT_SECRET is required")
	}
	if (cfg.Notify.TelegramBotToken == "") != (cfg.Notify.TelegramChatID == "") {
		return nil, errors.New("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must both be set or both empty")
	}
	if !cfg.Notify.HasDiscord() && !cfg.Notify.HasTelegram() {
		return nil, errors.New("at least one of DISCORD_WEBHOOK_URL or TELEGRAM_BOT_TOKEN+TELEGRAM_CHAT_ID is required")
	}

	return cfg, nil
}
