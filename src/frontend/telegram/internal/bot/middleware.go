package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func send(api *tgbotapi.BotAPI, chatID int64, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	api.Send(msg)
}

func editText(api *tgbotapi.BotAPI, chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewEditMessageText(chatID, msgID, text)
	msg.ReplyMarkup = kb
	api.Send(msg)
}

func sendOrEdit(api *tgbotapi.BotAPI, chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	if msgID != 0 {
		editText(api, chatID, msgID, text, kb)
	} else {
		send(api, chatID, text, kb)
	}
}

func backKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "back"),
		),
	)
}
