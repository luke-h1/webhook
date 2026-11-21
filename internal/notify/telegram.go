package notify

import (
	"fmt"
	"strings"
)

const (
	telegramTextLimit  = 4096
	telegramValueLimit = 512
)

type telegramLinkPreview struct {
	IsDisabled bool `json:"is_disabled"`
}

type telegramMessagePayload struct {
	ChatID             string              `json:"chat_id"`
	Text               string              `json:"text"`
	ParseMode          string              `json:"parse_mode"`
	LinkPreviewOptions telegramLinkPreview `json:"link_preview_options"`
}

func telegramPayload(chatID string, msg *Message) *telegramMessagePayload {
	return &telegramMessagePayload{
		ChatID:             chatID,
		Text:               msg.HTML(),
		ParseMode:          "HTML",
		LinkPreviewOptions: telegramLinkPreview{IsDisabled: true},
	}
}

func (m *Message) HTML() string {
	for shown := len(m.Sections); shown > 0; shown-- {
		if text := m.renderHTML(shown); runeLen(text) <= telegramTextLimit {
			return text
		}
	}
	return m.renderHTML(0)
}

func (m *Message) renderHTML(shownSections int) string {
	var b strings.Builder

	if header := m.Header(); header != "" {
		b.WriteString(withEmoji(m.Color, "<b>"+EscapeHTML(Truncate(header, telegramValueLimit))+"</b>"))
		b.WriteString("\n")
	}
	b.WriteString("<b>")
	b.WriteString(EscapeHTML(Truncate(m.Title, telegramValueLimit)))
	b.WriteString("</b>")

	for _, section := range m.Sections[:shownSections] {
		lines := telegramFieldLines(section.Fields)
		if section.Heading == "" {
			if len(lines) > 0 {
				b.WriteString("\n\n")
				b.WriteString(strings.Join(lines, "\n"))
			}
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(withEmoji(section.Color, "<b>"+EscapeHTML(Truncate(section.Heading, telegramValueLimit))+"</b>"))
		if len(lines) > 0 {
			b.WriteString("\n<blockquote>")
			b.WriteString(strings.Join(lines, "\n"))
			b.WriteString("</blockquote>")
		}
	}

	if hidden := len(m.Sections) - shownSections; hidden > 0 {
		fmt.Fprintf(&b, "\n\n<i>…and %d more</i>", hidden)
	}

	if m.Link != "" {
		fmt.Fprintf(&b, "\n\n<a href=\"%s\">%s →</a>", EscapeHTML(m.Link), EscapeHTML(m.linkText()))
	}
	return b.String()
}

func telegramFieldLines(fields []Field) []string {
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Value == "" {
			continue
		}
		value := EscapeHTML(Truncate(field.Value, telegramValueLimit))
		if field.Code {
			value = "<code>" + value + "</code>"
		}
		lines = append(lines, "<b>"+EscapeHTML(field.Label)+":</b> "+value)
	}
	return lines
}
