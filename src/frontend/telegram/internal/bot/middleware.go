package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func send(ctx context.Context, api *tgbotapi.BotAPI, chatID int64, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "send")
	defer span.End()

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	api.Send(msg)
}

func editText(ctx context.Context, api *tgbotapi.BotAPI, chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "editText")
	defer span.End()

	msg := tgbotapi.NewEditMessageText(chatID, msgID, text)
	msg.ReplyMarkup = kb
	api.Send(msg)
}

func sendOrEdit(ctx context.Context, api *tgbotapi.BotAPI, chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "sendOrEdit")
	defer span.End()

	if msgID != 0 {
		editText(ctx, api, chatID, msgID, text, kb)
	} else {
		send(ctx, api, chatID, text, kb)
	}
}

func backKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "back"),
		),
	)
}
