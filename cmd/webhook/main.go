package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/luke-h1/webhook/internal/webhook"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	cfg, err := webhook.LoadConfig()
	if err != nil {
		slog.Error("load webhook config", "err", err)
		os.Exit(1)
	}

	handler := webhook.NewHandler(cfg)
	lambda.Start(handler.HandleRequest)
}
