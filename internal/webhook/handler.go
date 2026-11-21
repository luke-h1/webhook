package webhook

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/luke-h1/webhook/internal/appeeky"
	"github.com/luke-h1/webhook/internal/notify"
	"github.com/luke-h1/webhook/internal/sentryhook"
)

const (
	maxBodyBytes = 6 << 20

	appeekyPath = "/api/appeeky"
	sentryPath  = "/api/sentry"
)

var errBodyTooLarge = errors.New("body too large")

type messageSender interface {
	Notify(ctx context.Context, msg *notify.Message) error
}

type Handler struct {
	cfg               *Config
	notifier          messageSender
	appeekyAPIKeyHash [32]byte
}

func NewHandler(cfg *Config) *Handler {
	return &Handler{
		cfg:               cfg,
		notifier:          notify.NewNotifier(&cfg.Notify),
		appeekyAPIKeyHash: sha256.Sum256([]byte(cfg.AppeekyAPIKey)),
	}
}

func (h *Handler) HandleRequest(ctx context.Context, req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	if req == nil {
		return errorResponse(http.StatusBadRequest, "invalid request"), nil
	}

	body, err := requestBody(req)
	if err != nil {
		slog.Warn("decode request body", "path", req.Path, "err", err)
		return errorResponse(http.StatusBadRequest, "invalid body"), nil
	}

	switch {
	case req.Path == appeekyPath && h.cfg.AppeekyAPIKey != "":
		return h.handleAppeeky(ctx, req, body), nil

	case req.Path == sentryPath && h.cfg.SentryClientSecret != "":
		return h.handleSentry(ctx, req, body), nil

	default:
		return errorResponse(http.StatusNotFound, "not found"), nil
	}
}

func (h *Handler) handleAppeeky(ctx context.Context, req *events.APIGatewayProxyRequest, body []byte) *events.APIGatewayProxyResponse {
	sum := sha256.Sum256([]byte(req.QueryStringParameters["key"]))

	if subtle.ConstantTimeCompare(sum[:], h.appeekyAPIKeyHash[:]) != 1 {
		return errorResponse(http.StatusUnauthorized, "unauthorized")
	}

	report, err := appeeky.ParseAlertPayload(body)
	if err != nil {
		slog.Warn("rejected appeeky payload", "err", err)
		return errorResponse(http.StatusBadRequest, "invalid payload")
	}

	slog.Info("appeeky alerts received", "app", report.App, "count", report.Count, "critical", report.Critical)

	if len(report.Unparsed) > 0 {
		slog.Warn("unparsed appeeky alert lines", "app", report.App, "lines", report.Unparsed)
	}

	if err := h.notifier.Notify(ctx, report.Message(h.cfg.Environment)); err != nil {
		slog.Error("notify appeeky alerts", "app", report.App, "err", err)
		return errorResponse(http.StatusBadGateway, "notify failed")
	}
	return jsonResponse(http.StatusOK, report)
}

func (h *Handler) handleSentry(ctx context.Context, req *events.APIGatewayProxyRequest, body []byte) *events.APIGatewayProxyResponse {
	if err := sentryhook.VerifySignature(body, header(req, "Sentry-Hook-Signature"), h.cfg.SentryClientSecret); err != nil {
		slog.Warn("rejected sentry webhook", "err", err)
		return errorResponse(http.StatusUnauthorized, "unauthorized")
	}

	resource := header(req, "Sentry-Hook-Resource")
	msg, err := sentryhook.ParseWebhook(resource, body)
	if err != nil {
		slog.Warn("parse sentry webhook", "resource", resource, "err", err)
		return errorResponse(http.StatusBadRequest, "invalid payload")
	}

	if msg == nil {
		slog.Info("skipping sentry webhook", "resource", resource)
		return jsonResponse(http.StatusOK, map[string]string{"status": "skipped"})
	}

	slog.Info("sentry webhook received", "resource", resource, "title", msg.Title)
	if err := h.notifier.Notify(ctx, msg); err != nil {
		slog.Error("notify sentry webhook", "resource", resource, "err", err)
		return errorResponse(http.StatusBadGateway, "notify failed")
	}
	return jsonResponse(http.StatusOK, map[string]string{"status": "sent"})
}

func header(req *events.APIGatewayProxyRequest, name string) string {
	for key, value := range req.Headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func requestBody(req *events.APIGatewayProxyRequest) ([]byte, error) {
	if len(req.Body) > maxBodyBytes {
		return nil, errBodyTooLarge
	}
	if !req.IsBase64Encoded {
		return []byte(req.Body), nil
	}
	return base64.StdEncoding.DecodeString(req.Body)
}

func errorResponse(status int, msg string) *events.APIGatewayProxyResponse {
	return jsonResponse(status, map[string]string{"error": msg})
}

func jsonResponse(status int, body any) *events.APIGatewayProxyResponse {
	raw, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		raw = []byte(`{"error":"internal server error"}`)
	}
	return &events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(raw),
	}
}
