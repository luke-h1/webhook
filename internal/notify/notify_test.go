package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testMessage() *Message {
	return &Message{
		Source: "Sentry",
		Kind:   "Alert",
		Title:  "a<b>",
		Color:  ColorRed,
		Sections: []Section{{Fields: []Field{
			{Label: "Level", Value: "error"},
			{Label: "Env", Value: ""},
			{Label: "Culprit", Value: "x & y_z", Code: true},
		}}},
		Link:     "https://luke-howsam.sentry.io/issues/1/?a=1&b=2",
		LinkText: "View in Sentry",
	}
}

func TestMessagePlainTextSkipsEmptyFields(t *testing.T) {
	got := testMessage().PlainText()
	want := "Sentry · Alert\na<b>\nLevel: error\nCulprit: x & y_z\n\nhttps://luke-howsam.sentry.io/issues/1/?a=1&b=2"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMessageHTML(t *testing.T) {
	got := testMessage().HTML()
	want := "🔴 <b>Sentry · Alert</b>\n<b>a&lt;b&gt;</b>\n\n<b>Level:</b> error\n<b>Culprit:</b> <code>x &amp; y_z</code>\n\n" +
		`<a href="https://luke-howsam.sentry.io/issues/1/?a=1&amp;b=2">View in Sentry →</a>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMessageHTMLHeadedSectionsUseBlockquote(t *testing.T) {
	msg := &Message{Source: "Appeeky", Title: "1 alert", Sections: []Section{{
		Heading: "Clicks fell",
		Color:   ColorYellow,
		Fields:  []Field{{Label: "Next", Value: "read summary"}},
	}}}
	want := "<b>Appeeky</b>\n<b>1 alert</b>\n\n🟡 <b>Clicks fell</b>\n<blockquote><b>Next:</b> read summary</blockquote>"
	if got := msg.HTML(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMessageHTMLDropsSectionsOverLimit(t *testing.T) {
	msg := &Message{Title: "many"}
	for range 40 {
		msg.Sections = append(msg.Sections, Section{
			Heading: "alert",
			Fields:  []Field{{Label: "Next", Value: strings.Repeat("x", telegramValueLimit)}},
		})
	}
	got := msg.HTML()
	if runeLen(got) > telegramTextLimit {
		t.Fatalf("length %d over limit", runeLen(got))
	}
	if !strings.Contains(got, "more</i>") || strings.Count(got, "<blockquote>") != strings.Count(got, "</blockquote>") {
		t.Fatalf("expected balanced tags and a hidden count, got %q", got[len(got)-80:])
	}
}

func TestDiscordPayload(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	got := discordPayload(testMessage(), now)
	want := &discordWebhookPayload{Embeds: []discordEmbed{{
		Author: &discordEmbedAuthor{Name: "🔴 Sentry · Alert"},
		Title:  "a<b>",
		URL:    "https://luke-howsam.sentry.io/issues/1/?a=1&b=2",
		Color:  ColorRed,
		Fields: []discordEmbedField{
			{Name: "Level", Value: "error", Inline: true},
			{Name: "Culprit", Value: "`x & y_z`"},
		},
		Footer:    &discordEmbedFooter{Text: "Sentry"},
		Timestamp: "2026-11-21T12:00:00Z",
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got.Embeds[0], want.Embeds[0])
	}
}

func TestDiscordPayloadHeadedSectionAndEscaping(t *testing.T) {
	msg := &Message{Title: "t", Sections: []Section{{
		Heading: "Dr. Sleep moved",
		Color:   ColorRed,
		Fields: []Field{
			{Label: "Numbers", Value: "climbed to #4 for *white_noise*"},
			{Label: "Source", Value: "a`b", Code: true},
		},
	}}}
	fields := discordPayload(msg, time.Now()).Embeds[0].Fields
	want := []discordEmbedField{{
		Name:  "🔴 Dr. Sleep moved",
		Value: "**Numbers:** climbed to #4 for \\*white\\_noise\\*\n**Source:** `` a`b ``",
	}}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("got %+v, want %+v", fields, want)
	}
}

func TestDiscordPayloadFitsEmbedLimits(t *testing.T) {
	msg := &Message{Title: "many"}
	for range 40 {
		msg.Sections = append(msg.Sections, Section{
			Heading: "alert",
			Fields:  []Field{{Label: "Next", Value: strings.Repeat("x", 600)}},
		})
	}
	embed := discordPayload(msg, time.Now()).Embeds[0]
	if len(embed.Fields) > discordFieldCount || discordEmbedSize(&embed) > discordEmbedTotalLimit {
		t.Fatalf("fields=%d size=%d", len(embed.Fields), discordEmbedSize(&embed))
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate(strings.Repeat("a", 10), 5); got != "aaaa…" {
		t.Fatalf("got %q", got)
	}
	if Truncate("short", 10) != "short" {
		t.Fatal("should not truncate short strings")
	}
}

func setTelegramBase(t *testing.T, url string) {
	t.Helper()
	prev := telegramAPIBaseURL
	telegramAPIBaseURL = url
	t.Cleanup(func() { telegramAPIBaseURL = prev })
}

func statusServer(t *testing.T, status int, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Errorf("ReadAll() error = %v", err)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNotify(t *testing.T) {
	tests := []struct {
		name           string
		discordStatus  int
		telegramStatus int
		wantErr        bool
	}{
		{name: "both channels deliver", discordStatus: http.StatusNoContent, telegramStatus: http.StatusOK},
		{name: "partial failure does not error", discordStatus: http.StatusNoContent, telegramStatus: http.StatusInternalServerError},
		{name: "all channels fail", discordStatus: http.StatusBadGateway, telegramStatus: http.StatusBadGateway, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var discordHits, telegramHits atomic.Int32
			discord := statusServer(t, tt.discordStatus, &discordHits)
			telegram := statusServer(t, tt.telegramStatus, &telegramHits)
			setTelegramBase(t, telegram.URL)

			n := NewNotifier(&Config{
				DiscordWebhookURL: discord.URL,
				TelegramBotToken:  "test-token",
				TelegramChatID:    "123",
			})
			err := n.Notify(t.Context(), testMessage())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Notify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if discordHits.Load() != 1 || telegramHits.Load() != 1 {
				t.Fatalf("discord=%d telegram=%d", discordHits.Load(), telegramHits.Load())
			}
		})
	}
}

func TestDoJSONRejectsRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("should not follow redirect")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer redirect.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, redirect.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doJSON(httpClient, req); err == nil {
		t.Fatal("expected error on redirect response")
	}
}

func TestDoJSONTransportErrorOmitsURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/botSECRET/sendMessage", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = doJSON(httpClient, req)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("got %v, want transport error without the URL", err)
	}
}
