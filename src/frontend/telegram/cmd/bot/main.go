package main

import (
	"context"
	"log/slog"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mralexandrov/debt-bot/frontend/telegram/internal/bot"
	observability "github.com/mralexandrov/go-observability"
)

func main() {
	token := mustEnv("TELEGRAM_BOT_TOKEN")
	backendAddr := envOr("BACKEND_ADDR", "backend:50051")

	ctx := context.Background()

	logger := observability.NewLogger("frontend")
	slog.SetDefault(logger)

	shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    "frontend",
		ServiceVersion: "0.1.0",
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "setup observability", "error", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	client, err := bot.NewClient(backendAddr)
	if err != nil {
		slog.ErrorContext(ctx, "create backend client", "error", err)
		os.Exit(1)
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		slog.ErrorContext(ctx, "create telegram bot", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "authorized on account", "username", api.Self.UserName)

	handler := bot.NewHandler(api, client)
	if err := handler.Run(); err != nil {
		slog.ErrorContext(ctx, "run bot", "error", err)
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
