package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.opentelemetry.io/otel/attribute"
)

// --- Message handler (text input) ---

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	ctx, span := tracer.Start(ctx, "handleMessage")
	defer span.End()

	tgID := msg.From.ID

	if msg.IsCommand() && msg.Command() == "start" {
		h.sm.Reset(tgID)
		user := h.resolveUser(ctx, msg.From)
		greeting := "Привет!"
		if user != nil {
			greeting = "Привет, " + user.Name + "!"
		}
		h.showMainMenu(ctx, msg.Chat.ID, 0, greeting+"\n\nЯ помогу рассчитать долги после совместных трат.")
		return
	}

	st := h.sm.Get(tgID)
	text := strings.TrimSpace(msg.Text)

	if st.step != stepIdle {
		span.SetAttributes(attribute.String("fsm.step", st.step))
	}

	switch st.step {
	case stepAwaitDealTitle:
		if text == "" {
			send(ctx, h.api, msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:", nil)
			return
		}
		user := h.resolveUser(ctx, msg.From)
		if user == nil {
			return
		}
		deal, err := h.client.CreateDeal(ctx, text, user.Id)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при создании сделки.", nil)
			return
		}
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ Сделка «%s» создана!", deal.Title), nil)
		h.showDealMenu(ctx, msg.Chat.ID, 0, deal.Id)

	case stepAwaitParticipantName:
		dealID := st.dealID
		newUser, notice, err := h.resolveParticipant(ctx, msg)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении участника.", nil)
			return
		}
		if newUser == nil {
			return
		}
		if _, err := h.client.AddDealParticipant(ctx, dealID, newUser.Id); err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении в сделку.", nil)
			return
		}
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ %s добавлен.%s", newUser.Name, notice), nil)
		h.showDealMenu(ctx, msg.Chat.ID, 0, dealID)

	case stepAwaitPurchaseTitle:
		if text == "" {
			send(ctx, h.api, msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:", nil)
			return
		}
		st.purchaseTitle = text
		st.step = stepAwaitPurchaseAmount
		kb := backKeyboard()
		send(ctx, h.api, msg.Chat.ID, "Введите сумму в рублях (например: 150 или 99.50 или 99,50):", &kb)

	case stepAwaitPurchaseAmount:
		amt, err := parseAmount(text)
		if err != nil || amt <= 0 {
			send(ctx, h.api, msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):", nil)
			return
		}
		st.purchaseAmt = amt

		deal, err := h.client.GetDeal(ctx, st.dealID)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при загрузке сделки.", nil)
			return
		}
		participants, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
		if err != nil || len(participants) == 0 {
			send(ctx, h.api, msg.Chat.ID, "Нет участников в сделке.", nil)
			return
		}
		for _, p := range participants {
			st.participantNames[p.Id] = p.Name
		}
		st.step = stepAwaitPurchasePayer
		h.showPayerKeyboard(ctx, msg.Chat.ID, 0, participants)

	case stepAwaitPurchasePayer:
		send(ctx, h.api, msg.Chat.ID, "Выберите плательщика из кнопок выше.", nil)

	case stepDealCovSelectPayer, stepDealCovSelectCovered:
		send(ctx, h.api, msg.Chat.ID, "Используйте кнопки для навигации.", nil)
	}
}
