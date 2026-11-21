package sentryhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/luke-h1/webhook/internal/notify"
)

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"action":"created"}`)
	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{name: "valid", signature: sign(body, "secret")},
		{name: "wrong secret", signature: sign(body, "other"), wantErr: true},
		{name: "not hex", signature: "zz", wantErr: true},
		{name: "missing", signature: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifySignature(body, tt.signature, "secret"); (err != nil) != tt.wantErr {
				t.Fatalf("VerifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	if err := VerifySignature(body, "", "secret"); !errors.Is(err, ErrMissingSignature) {
		t.Fatalf("got %v, want ErrMissingSignature", err)
	}
}

func TestParseWebhook(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		body      string
		wantTitle string
		wantColor int
		wantParts []string
	}{
		{
			name:      "issue alert",
			resource:  ResourceEventAlert,
			body:      `{"action":"triggered","data":{"triggered_rule":"Send to Discord","event":{"title":"ReferenceError: heck is not defined","web_url":"https://luke-howsam.sentry.io/issues/1117540176/events/abc/","issue_id":"1117540176","level":"error","culprit":"?(<anonymous>)","environment":"production","project":1,"platform":"javascript","release":null}}}`,
			wantTitle: "ReferenceError: heck is not defined",
			wantColor: notify.ColorRed,
			wantParts: []string{"Rule: Send to Discord", "Culprit: ?(<anonymous>)", "https://luke-howsam.sentry.io/issues/1117540176/events/abc/"},
		},
		{
			name:      "metric alert resolved",
			resource:  ResourceMetricAlert,
			body:      `{"action":"resolved","data":{"description_title":"Resolved: Too many errors","description_text":"1000 events in the last 10 minutes","web_url":"https://luke-howsam.sentry.io/alerts/1/","metric_alert":{"id":"1"}}}`,
			wantTitle: "Resolved: Too many errors",
			wantColor: notify.ColorGreen,
			wantParts: []string{"State: resolved", "Details: 1000 events in the last 10 minutes"},
		},
		{
			name:      "issue created",
			resource:  ResourceIssue,
			body:      `{"action":"created","data":{"issue":{"id":"1","shortId":"FOAM-Y","title":"TypeError: x is undefined","culprit":"app/index","level":"fatal","status":"unresolved","web_url":"https://luke-howsam.sentry.io/issues/1/","count":"3","userCount":2,"project":{"slug":"foam"}}}}`,
			wantTitle: "TypeError: x is undefined",
			wantColor: notify.ColorRed,
			wantParts: []string{"Sentry · New issue FOAM-Y", "Project: foam", "Events: 3", "Users: 2"},
		},
		{
			name:      "issue regressed",
			resource:  ResourceIssue,
			body:      `{"action":"unresolved","data":{"issue":{"shortId":"FOAM-Y","title":"TypeError","level":"warning","permalink":"https://luke-howsam.sentry.io/issues/1/"}}}`,
			wantTitle: "TypeError",
			wantColor: notify.ColorYellow,
			wantParts: []string{"Sentry · Regressed issue FOAM-Y", "https://luke-howsam.sentry.io/issues/1/"},
		},
		{
			name:      "error",
			resource:  ResourceError,
			body:      `{"action":"created","data":{"error":{"title":"ReferenceError: blooopy is not defined","web_url":"https://luke-howsam.sentry.io/issues/2/events/bb/","level":"error","culprit":"?(runner)","environment":"production","issue_id":"2","project":1,"release":null}}}`,
			wantTitle: "ReferenceError: blooopy is not defined",
			wantColor: notify.ColorRed,
			wantParts: []string{"Culprit: ?(runner)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseWebhook(tt.resource, []byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if msg == nil {
				t.Fatal("expected a message")
			}
			if msg.Title != tt.wantTitle || msg.Color != tt.wantColor {
				t.Fatalf("got title %q color %#x", msg.Title, msg.Color)
			}
			text := msg.PlainText()
			for _, part := range tt.wantParts {
				if !strings.Contains(text, part) {
					t.Errorf("message missing %q:\n%s", part, text)
				}
			}
		})
	}
}

func TestParseWebhookSkips(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		body     string
	}{
		{name: "installation", resource: ResourceInstallation, body: `{"action":"created","data":{}}`},
		{name: "issue resolved", resource: ResourceIssue, body: `{"action":"resolved","data":{"issue":{}}}`},
		{name: "issue alert outside production", resource: ResourceEventAlert, body: `{"action":"triggered","data":{"event":{"title":"boom","environment":"staging"}}}`},
		{name: "issue alert without environment", resource: ResourceEventAlert, body: `{"action":"triggered","data":{"event":{"title":"boom"}}}`},
		{name: "error outside production", resource: ResourceError, body: `{"action":"created","data":{"error":{"title":"boom","environment":"development"}}}`},
		{name: "unknown resource", resource: "comment", body: `{"action":"created","data":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseWebhook(tt.resource, []byte(tt.body))
			if err != nil || msg != nil {
				t.Fatalf("got %+v, %v; want nil, nil", msg, err)
			}
		})
	}
}

func TestParseWebhookInvalidJSON(t *testing.T) {
	if _, err := ParseWebhook(ResourceEventAlert, []byte("nope")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
