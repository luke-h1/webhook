package appeeky

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/luke-h1/webhook/internal/notify"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warn"
)

type Alert struct {
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Numbers  string   `json:"numbers"`
	Window   string   `json:"window"`
	Source   string   `json:"source"`
	Next     string   `json:"next"`
}

type Report struct {
	App      string   `json:"app"`
	Count    int      `json:"count"`
	Critical int      `json:"critical"`
	Alerts   []Alert  `json:"alerts"`
	Link     string   `json:"link,omitempty"`
	Unparsed []string `json:"unparsed,omitempty"`
}

var (
	ErrEmptyText  = errors.New("appeeky payload has no text")
	ErrNotAlerts  = errors.New("text is not appeeky spec alert message")
	headerPattern = regexp.MustCompile(`^\*Appeeky Spec · (.+)\*: (\d+) alerts?(?: \((\d+) critical\))?$`)
)

const (
	bulletPrefix   = "• "
	criticalPrefix = "*CRITICAL*: "
	warningPrefix  = "warning: "
	nextSeparator  = "). Next: "
	windowOpen     = " ("
	sourceSplit    = ", "
	titleSplit     = ". "
)

func ParseAlertPayload(body []byte) (*Report, error) {
	var payload struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse appeeky payload: %w", err)
	}
	return ParseAlertText(payload.Text)
}

func ParseAlertText(text string) (*Report, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if lines[0] == "" {
		return nil, ErrEmptyText
	}

	match := headerPattern.FindStringSubmatch(strings.TrimSpace(lines[0]))

	if match == nil {
		return nil, ErrNotAlerts
	}

	report := &Report{App: match[1]}
	report.Count, _ = strconv.Atoi(match[2])

	if match[3] != "" {
		report.Critical, _ = strconv.Atoi(match[3])
	}

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, bulletPrefix):
			alert, ok := parseAlertLine(strings.TrimPrefix(line, bulletPrefix))
			if !ok {
				report.Unparsed = append(report.Unparsed, line)
				continue
			}
			report.Alerts = append(report.Alerts, alert)

		case strings.HasPrefix(line, "https://"), strings.HasPrefix(line, "http://"):
			report.Link = line
		default:
			report.Unparsed = append(report.Unparsed, line)
		}
	}
	return report, nil
}

func parseAlertLine(line string) (Alert, bool) {
	var alert Alert
	switch {
	case strings.HasPrefix(line, criticalPrefix):
		alert.Severity, line = SeverityCritical, strings.TrimPrefix(line, criticalPrefix)
	case strings.HasPrefix(line, warningPrefix):
		alert.Severity, line = SeverityWarning, strings.TrimPrefix(line, warningPrefix)
	default:
		return alert, false
	}

	head, next, ok := strings.Cut(line, nextSeparator)
	if !ok {
		return alert, false
	}
	alert.Next = next

	summary, context, ok := strings.CutLast(head, windowOpen)
	if !ok {
		return alert, false
	}
	if alert.Window, alert.Source, ok = strings.CutLast(context, sourceSplit); !ok {
		return alert, false
	}
	if alert.Title, alert.Numbers, ok = strings.CutLast(summary, titleSplit); !ok {
		return alert, false
	}
	return alert, true
}

func (r *Report) Message(env string) *notify.Message {
	color := notify.ColorYellow
	if r.Critical > 0 {
		color = notify.ColorRed
	}

	sections := []notify.Section{{Fields: []notify.Field{{Label: "Env", Value: env}}}}

	for _, alert := range r.Alerts {
		sections = append(sections, notify.Section{
			Heading: alert.Title,
			Color:   alert.Severity.color(),
			Fields: []notify.Field{
				{Label: "Numbers", Value: alert.Numbers},
				{Label: "Window", Value: alert.Window},
				{Label: "Source", Value: alert.Source, Code: true},
				{Label: "Next", Value: alert.Next},
			},
		})
	}
	return &notify.Message{
		Source:   "Appeeky",
		Kind:     r.App,
		Title:    fmt.Sprintf("%d alerts (%d critical)", r.Count, r.Critical),
		Color:    color,
		Sections: sections,
		Link:     r.Link,
		LinkText: "View report",
	}
}

func (s Severity) color() int {
	if s == SeverityCritical {
		return notify.ColorRed
	}

	return notify.ColorYellow
}
