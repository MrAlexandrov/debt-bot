package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mrralexandrov/debt-bot/frontend/telegram/internal/bot"
)

func main() {
	token := mustEnv("TELEGRAM_BOT_TOKEN")
	backendAddr := envOr("BACKEND_ADDR", "backend:50051")

	client, err := bot.NewClient(backendAddr)
	if err != nil {
		log.Fatalf("create backend client: %v", err)
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("create telegram bot: %v", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	handler := bot.NewHandler(api, client)
	if err := handler.Run(); err != nil {
		log.Fatalf("run bot: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
