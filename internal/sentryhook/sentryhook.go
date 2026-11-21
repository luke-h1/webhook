package sentryhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/luke-h1/webhook/internal/notify"
)

const (
	ResourceEventAlert   = "event_alert"
	ResourceMetricAlert  = "metric_alert"
	ResourceIssue        = "issue"
	ResourceError        = "error"
	ResourceInstallation = "installation"
)

var ErrMissingSignature = errors.New("sentry webhook missing signature")

type envelope struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type eventAlertData struct {
	TriggeredRule string `json:"triggered_rule"`
	Event         struct {
		Title       string `json:"title"`
		WebURL      string `json:"web_url"`
		IssueID     string `json:"issue_id"`
		Level       string `json:"level"`
		Culprit     string `json:"culprit"`
		Environment string `json:"environment"`
		Release     string `json:"release"`
		Platform    string `json:"platform"`
	} `json:"event"`
}

type metricAlertData struct {
	DescriptionTitle string `json:"description_title"`
	DescriptionText  string `json:"description_text"`
	WebURL           string `json:"web_url"`
}

type issueData struct {
	Issue struct {
		ShortID   string `json:"shortId"`
		Title     string `json:"title"`
		Culprit   string `json:"culprit"`
		Level     string `json:"level"`
		Status    string `json:"status"`
		WebURL    string `json:"web_url"`
		Permalink string `json:"permalink"`
		Count     string `json:"count"`
		UserCount int    `json:"userCount"`
		FirstSeen string `json:"firstSeen"`
		LastSeen  string `json:"lastSeen"`
		Project   struct {
			Slug string `json:"slug"`
		} `json:"project"`
	} `json:"issue"`
}

type errorData struct {
	Error struct {
		Title       string `json:"title"`
		WebURL      string `json:"web_url"`
		IssueID     string `json:"issue_id"`
		Level       string `json:"level"`
		Culprit     string `json:"culprit"`
		Environment string `json:"environment"`
		Release     string `json:"release"`
	} `json:"error"`
}

func VerifySignature(body []byte, signature, clientSecret string) error {
	if signature == "" {
		return ErrMissingSignature
	}

	got, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode sentry signature: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return errors.New("sentry signature mismatch")
	}
	return nil
}

func ParseWebhook(resource string, body []byte) (*notify.Message, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse sentry webhook: %w", err)
	}

	switch resource {
	case ResourceEventAlert:
		var data eventAlertData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("parse sentry event alert: %w", err)
		}
		return eventAlertMessage(&data), nil
	case ResourceMetricAlert:
		var data metricAlertData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("parse sentry metric alert: %w", err)
		}
		return metricAlertMessage(env.Action, &data), nil
	case ResourceIssue:
		if env.Action != "created" && env.Action != "unresolved" {
			return nil, nil
		}
		var data issueData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("parse sentry issue: %w", err)
		}
		return issueMessage(env.Action, &data), nil
	case ResourceError:
		var data errorData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("parse sentry error: %w", err)
		}
		return errorMessage(&data), nil
	default:
		return nil, nil
	}
}

const (
	source   = "Sentry"
	linkText = "View in Sentry"
)

func eventAlertMessage(data *eventAlertData) *notify.Message {
	event := data.Event
	return &notify.Message{
		Source: source,
		Kind:   "Alert",
		Title:  event.Title,
		Color:  levelColor(event.Level),
		Sections: []notify.Section{{Fields: []notify.Field{
			{Label: "Level", Value: event.Level},
			{Label: "Env", Value: event.Environment},
			{Label: "Platform", Value: event.Platform},
			{Label: "Rule", Value: data.TriggeredRule},
			{Label: "Culprit", Value: event.Culprit, Code: true},
			{Label: "Release", Value: event.Release, Code: true},
		}}},
		Link:     event.WebURL,
		LinkText: linkText,
	}
}

func metricAlertMessage(action string, data *metricAlertData) *notify.Message {
	color := notify.ColorYellow
	switch action {
	case "critical":
		color = notify.ColorRed
	case "resolved":
		color = notify.ColorGreen
	}
	return &notify.Message{
		Source: source,
		Kind:   "Metric alert",
		Title:  data.DescriptionTitle,
		Color:  color,
		Sections: []notify.Section{{Fields: []notify.Field{
			{Label: "State", Value: action},
			{Label: "Details", Value: data.DescriptionText},
		}}},
		Link:     data.WebURL,
		LinkText: linkText,
	}
}

func issueMessage(action string, data *issueData) *notify.Message {
	issue := data.Issue
	kind := "New issue"
	if action == "unresolved" {
		kind = "Regressed issue"
	}
	if issue.ShortID != "" {
		kind += " " + issue.ShortID
	}
	link := issue.WebURL
	if link == "" {
		link = issue.Permalink
	}
	users := ""
	if issue.UserCount > 0 {
		users = strconv.Itoa(issue.UserCount)
	}
	return &notify.Message{
		Source: source,
		Kind:   kind,
		Title:  issue.Title,
		Color:  levelColor(issue.Level),
		Sections: []notify.Section{{Fields: []notify.Field{
			{Label: "Project", Value: issue.Project.Slug},
			{Label: "Level", Value: issue.Level},
			{Label: "Events", Value: issue.Count},
			{Label: "Users", Value: users},
			{Label: "First seen", Value: formatTime(issue.FirstSeen)},
			{Label: "Last seen", Value: formatTime(issue.LastSeen)},
			{Label: "Culprit", Value: issue.Culprit, Code: true},
		}}},
		Link:     link,
		LinkText: linkText,
	}
}

func errorMessage(data *errorData) *notify.Message {
	sentryErr := data.Error
	return &notify.Message{
		Source: source,
		Kind:   "Error",
		Title:  sentryErr.Title,
		Color:  levelColor(sentryErr.Level),
		Sections: []notify.Section{{Fields: []notify.Field{
			{Label: "Level", Value: sentryErr.Level},
			{Label: "Env", Value: sentryErr.Environment},
			{Label: "Culprit", Value: sentryErr.Culprit, Code: true},
			{Label: "Release", Value: sentryErr.Release, Code: true},
		}}},
		Link:     sentryErr.WebURL,
		LinkText: linkText,
	}
}

func formatTime(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format("2 Jan 2006 15:04 UTC")
}

func levelColor(level string) int {
	switch strings.ToLower(level) {
	case "fatal", "error":
		return notify.ColorRed
	default:
		return notify.ColorYellow
	}
}
