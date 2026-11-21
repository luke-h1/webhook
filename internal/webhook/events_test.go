package webhook

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestEventFixtures(t *testing.T) {
	root, err := os.OpenRoot("../../events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	files, err := filepath.Glob("../../events/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no event fixtures found")
	}

	localSecrets := map[string]string{
		"APPEEKY_API_KEY":      "local-appeeky-key",
		"SENTRY_CLIENT_SECRET": "local-sentry-secret",
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := root.ReadFile(filepath.Base(file))
			if err != nil {
				t.Fatal(err)
			}
			var req events.APIGatewayProxyRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatal(err)
			}

			sender := &fakeSender{}
			resp, err := newTestHandler(t, sender, localSecrets).HandleRequest(t.Context(), &req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK || len(sender.sent) != 1 {
				t.Fatalf("status %d, sent %d: %s", resp.StatusCode, len(sender.sent), resp.Body)
			}
		})
	}
}
