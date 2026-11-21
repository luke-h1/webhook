package notify

import (
	"strings"
	"time"
)

// https://discord.com/developers/docs/resources/message#embed-object-embed-limits
const (
	discordTitleLimit      = 256
	discordAuthorLimit     = 256
	discordFieldNameLimit  = 256
	discordFieldValueLimit = 1024
	discordFieldCount      = 25
	discordEmbedTotalLimit = 6000
)

type discordEmbedAuthor struct {
	Name string `json:"name"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

type discordEmbed struct {
	Author    *discordEmbedAuthor `json:"author,omitempty"`
	Title     string              `json:"title"`
	URL       string              `json:"url,omitempty"`
	Color     int                 `json:"color"`
	Fields    []discordEmbedField `json:"fields,omitempty"`
	Footer    *discordEmbedFooter `json:"footer,omitempty"`
	Timestamp string              `json:"timestamp"`
}

type discordWebhookPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

func discordPayload(msg *Message, now time.Time) *discordWebhookPayload {
	embed := discordEmbed{
		Title:     Truncate(msg.Title, discordTitleLimit),
		URL:       msg.Link,
		Color:     msg.Color,
		Fields:    discordFields(msg.Sections),
		Timestamp: now.UTC().Format(time.RFC3339),
	}

	if header := msg.Header(); header != "" {
		embed.Author = &discordEmbedAuthor{Name: Truncate(withEmoji(msg.Color, header), discordAuthorLimit)}
	}

	if msg.Source != "" {
		embed.Footer = &discordEmbedFooter{Text: msg.Source}
	}
	fitDiscordEmbed(&embed)
	return &discordWebhookPayload{Embeds: []discordEmbed{embed}}
}

func discordFields(sections []Section) []discordEmbedField {
	var fields []discordEmbedField

	for _, section := range sections {
		if section.Heading == "" {
			for _, field := range section.Fields {
				if field.Value == "" {
					continue
				}

				fields = append(fields, discordEmbedField{
					Name:   Truncate(field.Label, discordFieldNameLimit),
					Value:  discordValue(field, discordFieldValueLimit),
					Inline: !field.Code,
				})
			}
			continue
		}

		lines := make([]string, 0, len(section.Fields))
		for _, field := range section.Fields {
			if field.Value == "" {
				continue
			}
			lines = append(lines, "**"+escapeMarkdown(field.Label)+":** "+discordValue(field, discordFieldValueLimit/2))
		}

		fields = append(fields, discordEmbedField{
			Name:  Truncate(withEmoji(section.Color, section.Heading), discordFieldNameLimit),
			Value: Truncate(strings.Join(lines, "\n"), discordFieldValueLimit),
		})
	}
	return fields
}

func discordValue(field Field, limit int) string {
	if !field.Code {
		return Truncate(escapeMarkdown(field.Value), limit)
	}

	value := Truncate(field.Value, limit-6)
	if strings.Contains(value, "`") {
		return "`` " + value + " ``"
	}
	return "`" + value + "`"
}

func fitDiscordEmbed(embed *discordEmbed) {
	if len(embed.Fields) > discordFieldCount {
		embed.Fields = embed.Fields[:discordFieldCount]
	}
	for len(embed.Fields) > 0 && discordEmbedSize(embed) > discordEmbedTotalLimit {
		embed.Fields = embed.Fields[:len(embed.Fields)-1]
	}
}

func discordEmbedSize(embed *discordEmbed) int {
	size := runeLen(embed.Title)
	if embed.Author != nil {
		size += runeLen(embed.Author.Name)
	}
	if embed.Footer != nil {
		size += runeLen(embed.Footer.Text)
	}
	for _, field := range embed.Fields {
		size += runeLen(field.Name) + runeLen(field.Value)
	}
	return size
}

func escapeMarkdown(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"*", `\*`,
		"_", `\_`,
		"~", `\~`,
		"`", "\\`",
		"|", `\|`,
		">", `\>`,
	)
	return replacer.Replace(value)
}

func runeLen(s string) int {
	return len([]rune(s))
}
