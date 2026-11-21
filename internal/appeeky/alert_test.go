package appeeky

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const samplePayload = `*Appeeky Spec · Foam*: 3 alerts (1 critical)
• *CRITICAL*: Release 2026-11-02 lost on day 7 (subtitle: white noise). decided by downloads; 1 won, 2 lost (2026-11-02 → 2026-11-09, appeeky/releases/2026-11-02.yaml). Next: A person decides: revert 2026-11-02, then push (skill appeeky-spec-review).
• warning: Google search clicks fell 40%. 120 clicks in 7 days to 2026-11-19 (17.1/day), 200 in the 7 days before (28.6/day) (2026-11-06 → 2026-11-19, appeeky/seo/daily.csv). Next: Read appeeky/seo/summary.md for the queries and pages that fell, and run seo check (skill appeeky-spec-seo).
• warning: Dr. Sleep moved on your targets. climbed to #4 for "white noise" (2026-11-18, appeeky/competitors/changes.md). Next: Read appeeky/competitors/dr-sleep.yaml: its listing and the searches it ranks for (skill appeeky-spec-competitors).
https://github.com/luke-h1/foam/issues/12`

func TestParseAlertText(t *testing.T) {
	got, err := ParseAlertText(samplePayload)
	if err != nil {
		t.Fatal(err)
	}
	want := &Report{
		App:      "Foam",
		Count:    3,
		Critical: 1,
		Link:     "https://github.com/luke-h1/foam/issues/12",
		Alerts: []Alert{
			{
				Severity: SeverityCritical,
				Title:    "Release 2026-11-02 lost on day 7 (subtitle: white noise)",
				Numbers:  "decided by downloads; 1 won, 2 lost",
				Window:   "2026-11-02 → 2026-11-09",
				Source:   "appeeky/releases/2026-11-02.yaml",
				Next:     "A person decides: revert 2026-11-02, then push (skill appeeky-spec-review).",
			},
			{
				Severity: SeverityWarning,
				Title:    "Google search clicks fell 40%",
				Numbers:  "120 clicks in 7 days to 2026-11-19 (17.1/day), 200 in the 7 days before (28.6/day)",
				Window:   "2026-11-06 → 2026-11-19",
				Source:   "appeeky/seo/daily.csv",
				Next:     "Read appeeky/seo/summary.md for the queries and pages that fell, and run seo check (skill appeeky-spec-seo).",
			},
			{
				Severity: SeverityWarning,
				Title:    "Dr. Sleep moved on your targets",
				Numbers:  `climbed to #4 for "white noise"`,
				Window:   "2026-11-18",
				Source:   "appeeky/competitors/changes.md",
				Next:     "Read appeeky/competitors/dr-sleep.yaml: its listing and the searches it ranks for (skill appeeky-spec-competitors).",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAlertText() mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseAlertTextSingleWarning(t *testing.T) {
	got, err := ParseAlertText("*Appeeky Spec · Foam*: 1 alert\n• warning: A wave of 1–2 star reviews on the App Store. 6 in the last 7 days, 1 in the 7 days before; 4 unanswered (2026-11-06 → 2026-11-20, appeeky/reviews/app-store). Next: Read them (skill appeeky-spec-reviews).")
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Critical != 0 || len(got.Alerts) != 1 || len(got.Unparsed) != 0 {
		t.Fatalf("unexpected report: %+v", got)
	}
}

func TestParseAlertTextKeepsUnknownLines(t *testing.T) {
	got, err := ParseAlertText("*Appeeky Spec · Foam*: 1 alert\n• something new")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Alerts) != 0 || !reflect.DeepEqual(got.Unparsed, []string{"• something new"}) {
		t.Fatalf("unexpected report: %+v", got)
	}
}

func TestParseAlertPayloadRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{name: "empty text", body: `{"text":""}`, want: ErrEmptyText},
		{name: "other sender", body: `{"text":"hello"}`, want: ErrNotAlerts},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseAlertPayload([]byte(tt.body)); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
	if _, err := ParseAlertPayload([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReportMessage(t *testing.T) {
	report, err := ParseAlertText(samplePayload)
	if err != nil {
		t.Fatal(err)
	}
	msg := report.Message("production")
	text := msg.PlainText()
	for _, part := range []string{"Appeeky · Foam\n3 alerts (1 critical)", "Env: production", "\n\nRelease 2026-11-02 lost on day 7", "Source: appeeky/seo/daily.csv", report.Link} {
		if !strings.Contains(text, part) {
			t.Errorf("message missing %q:\n%s", part, text)
		}
	}
}
