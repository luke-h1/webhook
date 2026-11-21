package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/luke-h1/webhook/internal/notify"
)

const appeekyBody = `{"text":"*Appeeky Spec · Foam*: 1 alert\n• warning: Google search clicks fell 40%. 120 clicks (2026-11-06 → 2026-11-19, appeeky/seo/daily.csv). Next: Run seo check."}`

const sentryBody = `{"action":"triggered","data":{"triggered_rule":"r","event":{"title":"boom","level":"error","environment":"production"}}}`

type fakeSender struct {
	err  error
	sent []*notify.Message
}

func (f *fakeSender) Notify(_ context.Context, msg *notify.Message) error {
	f.sent = append(f.sent, msg)
	return f.err
}

func newTestHandler(t *testing.T, sender *fakeSender, env map[string]string) *Handler {
	t.Helper()
	for _, key := range []string{"APPEEKY_API_KEY", "SENTRY_CLIENT_SECRET", "ENVIRONMENT", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
		t.Setenv(key, env[key])
	}
	t.Setenv("DISCORD_WEBHOOK_URL", "https://discord.example/webhook")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(cfg)
	h.notifier = sender
	return h
}

var bothSources = map[string]string{
	"APPEEKY_API_KEY":      "appeeky-key",
	"SENTRY_CLIENT_SECRET": "sentry-secret",
	"ENVIRONMENT":          "staging",
}

func sign(body string) string {
	mac := hmac.New(sha256.New, []byte("sentry-secret"))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHandleRequest(t *testing.T) {
	tests := []struct {
		name       string
		request    *events.APIGatewayProxyRequest
		notifyErr  error
		statusCode int
		wantSent   int
		bodyPart   string
	}{
		{
			name:       "appeeky alerts notify",
			request:    &events.APIGatewayProxyRequest{Path: appeekyPath, QueryStringParameters: map[string]string{"key": "appeeky-key"}, Body: appeekyBody},
			statusCode: http.StatusOK,
			wantSent:   1,
			bodyPart:   `"app":"Foam"`,
		},
		{
			name:       "appeeky wrong key",
			request:    &events.APIGatewayProxyRequest{Path: appeekyPath, QueryStringParameters: map[string]string{"key": "nope"}, Body: appeekyBody},
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "appeeky other payload",
			request:    &events.APIGatewayProxyRequest{Path: appeekyPath, QueryStringParameters: map[string]string{"key": "appeeky-key"}, Body: `{"text":"hi"}`},
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "appeeky notify failure asks for a retry",
			request:    &events.APIGatewayProxyRequest{Path: appeekyPath, QueryStringParameters: map[string]string{"key": "appeeky-key"}, Body: appeekyBody},
			notifyErr:  errors.New("down"),
			statusCode: http.StatusBadGateway,
			wantSent:   1,
		},
		{
			name: "sentry issue alert notifies",
			request: &events.APIGatewayProxyRequest{Path: sentryPath, Body: sentryBody, Headers: map[string]string{
				"sentry-hook-resource":  "event_alert",
				"sentry-hook-signature": sign(sentryBody),
			}},
			statusCode: http.StatusOK,
			wantSent:   1,
			bodyPart:   `"status":"sent"`,
		},
		{
			name: "sentry bad signature",
			request: &events.APIGatewayProxyRequest{Path: sentryPath, Body: sentryBody, Headers: map[string]string{
				"Sentry-Hook-Resource":  "event_alert",
				"Sentry-Hook-Signature": strings.Repeat("0", 64),
			}},
			statusCode: http.StatusUnauthorized,
		},
		{
			name: "sentry installation is skipped",
			request: &events.APIGatewayProxyRequest{Path: sentryPath, Body: sentryBody, Headers: map[string]string{
				"Sentry-Hook-Resource":  "installation",
				"Sentry-Hook-Signature": sign(sentryBody),
			}},
			statusCode: http.StatusOK,
			bodyPart:   `"status":"skipped"`,
		},
		{
			name:       "unknown path",
			request:    &events.APIGatewayProxyRequest{Path: "/api/nope"},
			statusCode: http.StatusNotFound,
		},
		{
			name:       "nil request",
			statusCode: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeSender{err: tt.notifyErr}
			resp, err := newTestHandler(t, sender, bothSources).HandleRequest(t.Context(), tt.request)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.statusCode {
				t.Fatalf("status %d, want %d: %s", resp.StatusCode, tt.statusCode, resp.Body)
			}
			if len(sender.sent) != tt.wantSent {
				t.Fatalf("sent %d messages, want %d", len(sender.sent), tt.wantSent)
			}
			if tt.bodyPart != "" && !strings.Contains(resp.Body, tt.bodyPart) {
				t.Fatalf("body %q missing %q", resp.Body, tt.bodyPart)
			}
		})
	}
}

func TestHandleRequestDisabledSource(t *testing.T) {
	h := newTestHandler(t, &fakeSender{}, map[string]string{"APPEEKY_API_KEY": "appeeky-key"})
	resp, err := h.HandleRequest(t.Context(), &events.APIGatewayProxyRequest{Path: sentryPath, Body: sentryBody})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404 when SENTRY_CLIENT_SECRET is unset", resp.StatusCode)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{name: "no source", env: map[string]string{"DISCORD_WEBHOOK_URL": "https://discord.example"}, wantErr: true},
		{name: "no destination", env: map[string]string{"APPEEKY_API_KEY": "k"}, wantErr: true},
		{name: "telegram half set", env: map[string]string{"SENTRY_CLIENT_SECRET": "s", "TELEGRAM_BOT_TOKEN": "t"}, wantErr: true},
		{name: "sentry to discord", env: map[string]string{"SENTRY_CLIENT_SECRET": "s", "DISCORD_WEBHOOK_URL": "https://discord.example"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"APPEEKY_API_KEY", "SENTRY_CLIENT_SECRET", "DISCORD_WEBHOOK_URL", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
				t.Setenv(key, tt.env[key])
			}
			if _, err := LoadConfig(); (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
