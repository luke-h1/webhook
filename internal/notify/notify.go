package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	httpTimeout = 5 * time.Second
	ColorRed    = 0xed4245
	ColorYellow = 0xfee75c
	ColorGreen  = 0x57f287
)

var (
	httpClient = &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	telegramAPIBaseURL = "https://api.telegram.org"
)

type Field struct {
	Label string
	Value string
	Code  bool
}

type Section struct {
	Heading string
	Color   int
	Fields  []Field
}

type Message struct {
	Source   string
	Kind     string
	Title    string
	Color    int
	Sections []Section
	Link     string
	LinkText string
}

func (m *Message) Header() string {
	if m.Kind == "" {
		return m.Source
	}
	return m.Source + " · " + m.Kind
}

func (m *Message) linkText() string {
	if m.LinkText != "" {
		return m.LinkText
	}
	return "Open"
}

func (m *Message) PlainText() string {
	var b strings.Builder

	b.WriteString(m.Header())
	b.WriteString("\n")
	b.WriteString(m.Title)

	for _, section := range m.Sections {
		if section.Heading != "" {
			b.WriteString("\n\n")
			b.WriteString(section.Heading)
		}

		for _, field := range section.Fields {
			if field.Value == "" {
				continue
			}
			b.WriteString("\n")
			b.WriteString(field.Label)
			b.WriteString(": ")
			b.WriteString(field.Value)
		}
	}
	if m.Link != "" {
		b.WriteString("\n\n")
		b.WriteString(m.Link)
	}
	return b.String()
}

func colorEmoji(color int) string {
	switch color {
	case ColorRed:
		return "🔴"
	case ColorYellow:
		return "🟡"
	case ColorGreen:
		return "🟢"
	default:
		return ""
	}
}

func withEmoji(color int, text string) string {
	if emoji := colorEmoji(color); emoji != "" {
		return emoji + " " + text
	}
	return text
}

type Config struct {
	DiscordWebhookURL string
	TelegramBotToken  string
	TelegramChatID    string
}

func (c *Config) HasDiscord() bool {
	return c.DiscordWebhookURL != ""
}

func (c *Config) HasTelegram() bool {
	return c.TelegramBotToken != "" && c.TelegramChatID != ""
}

type Notifier struct {
	cfg    *Config
	client *http.Client
}

func NewNotifier(cfg *Config) *Notifier {
	return &Notifier{cfg: cfg, client: httpClient}
}

func (n *Notifier) Notify(ctx context.Context, msg *Message) error {
	type result struct {
		name string
		err  error
	}

	var (
		wg      sync.WaitGroup
		results = make(chan result, 2)
		sent    atomic.Int32
	)

	if n.cfg.HasDiscord() {
		wg.Go(func() {
			if err := n.sendDiscord(ctx, msg); err != nil {
				results <- result{name: "discord", err: err}
				return
			}
			sent.Add(1)
		})
	}

	if n.cfg.HasTelegram() {
		wg.Go(func() {
			if err := n.sendTelegram(ctx, msg); err != nil {
				results <- result{name: "telegram", err: err}
				return
			}
			sent.Add(1)
		})
	}
	wg.Wait()
	close(results)

	errs := []error{}

	for r := range results {
		errs = append(errs, fmt.Errorf("%s: %w", r.name, r.err))
	}

	if len(errs) == 0 {
		return nil
	}

	joined := errors.Join(errs...)

	if sent.Load() > 0 {
		slog.Warn("partial notify failure", "err", joined)
		return nil
	}
	return fmt.Errorf("notify failed: %w", joined)
}

func (n *Notifier) sendDiscord(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(discordPayload(msg, time.Now()))
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.DiscordWebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if err := doJSON(n.client, req); err != nil {
		return fmt.Errorf("send discord webhook: %w", err)
	}

	return nil
}

func (n *Notifier) sendTelegram(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(telegramPayload(n.cfg.TelegramChatID, msg))
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBaseURL, n.cfg.TelegramBotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if err := doJSON(n.client, req); err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	return nil
}

func doJSON(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return fmt.Errorf("%s request: %w", urlErr.Op, urlErr.Err)
		}
		return err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("close response body", "err", closeErr)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("unexpected status %d: read body: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func EscapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(value)
}

func Truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-1]) + "…"
}
