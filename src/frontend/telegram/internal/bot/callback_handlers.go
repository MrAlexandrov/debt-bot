package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- Callback handler (button presses) ---

func (h *Handler) handleCallback(cb *tgbotapi.CallbackQuery) {
	ctx := context.Background()
	tgID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	h.api.Request(tgbotapi.NewCallback(cb.ID, ""))

	data := cb.Data

	exactHandlers := map[string]func(){
		"main_menu":     func() { h.handleMainMenu(ctx, tgID, chatID, msgID) },
		"new_deal":      func() { h.handleNewDeal(ctx, tgID, chatID, msgID) },
		"my_deals":      func() { h.handleMyDeals(ctx, tgID, chatID, msgID, cb) },
		"deal_cov_add":  func() { h.handleDealCoverageAdd(ctx, tgID, chatID, msgID) },
		"deal_cov_back": func() { h.handleDealCoverageBack(ctx, tgID, chatID, msgID) },
		"back":          func() { h.handleBack(ctx, tgID, chatID, msgID) },
	}

	if handler, ok := exactHandlers[data]; ok {
		handler()
		return
	}

	prefixHandlers := []struct {
		prefix  string
		handler func()
	}{
		{"deal:", func() { h.handleDeal(ctx, tgID, chatID, msgID, cb) }},
		{"add_participant:", func() { h.handleAddParticipant(ctx, tgID, chatID, msgID, cb) }},
		{"add_purchase:", func() { h.handleAddPurchase(ctx, tgID, chatID, msgID, cb) }},
		{"purchases:", func() { h.handlePurchases(chatID, msgID, cb) }},
		{"calculate:", func() { h.handleCalculate(chatID, msgID, cb) }},
		{"deal_coverages:", func() { h.handleDealCoverages(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_payer:", func() { h.handleDealCoveragePayer(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_covered:", func() { h.handleDealCoverageCovered(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_remove:", func() { h.handleDealCoverageRemove(ctx, tgID, chatID, msgID, cb) }},
		{"payer:", func() { h.handleCreatePurchase(ctx, tgID, chatID, msgID, cb) }},
	}

	for _, ph := range prefixHandlers {
		if strings.HasPrefix(data, ph.prefix) {
			ph.handler()
			return
		}
	}

	editText(h.api, chatID, msgID, "Неизвестная команда", nil)
}

func (h *Handler) handleMainMenu(_ context.Context, tgID, chatID int64, msgID int) {
	h.resetState(tgID)
	h.showMainMenu(chatID, msgID, "Главное меню:")
}

func (h *Handler) handleNewDeal(_ context.Context, tgID, chatID int64, msgID int) {
	h.getState(tgID).step = stepAwaitDealTitle
	kb := backKeyboard()
	editText(h.api, chatID, msgID, "Введите название сделки:", &kb)
}

func (h *Handler) handleMyDeals(ctx context.Context, _, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	user := h.resolveUserFromCB(ctx, cb.From)
	if user == nil {
		return
	}
	h.showDealsList(chatID, msgID, user.Id)
}

func (h *Handler) handleDeal(_ context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "deal:")
	h.resetState(tgID)
	h.showDealMenu(chatID, msgID, dealID)
}

func (h *Handler) handleAddParticipant(_ context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "add_participant:")
	st := h.getState(tgID)
	st.step = stepAwaitParticipantName
	st.dealID = dealID
	kb := backKeyboard()
	editText(h.api, chatID, msgID, "Добавьте участника одним из способов:\n\n• Введите имя\n• Отправьте @username\n• Перешлите сообщение от участника", &kb)
}

func (h *Handler) handleAddPurchase(_ context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "add_purchase:")
	st := h.getState(tgID)
	st.step = stepAwaitPurchaseTitle
	st.dealID = dealID
	kb := backKeyboard()
	editText(h.api, chatID, msgID, "Введите название покупки:", &kb)
}

func (h *Handler) handlePurchases(chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "purchases:")
	h.showPurchases(chatID, msgID, dealID)
}

func (h *Handler) handleCalculate(chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "calculate:")
	h.showCalculation(chatID, msgID, dealID)
}

// Deal-level coverages management screen
// "deal_coverages:{dealID}" → max 15+36=51 chars ✓
func (h *Handler) handleDealCoverages(_ context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	dealID := strings.TrimPrefix(cb.Data, "deal_coverages:")
	st := h.getState(tgID)
	st.dealID = dealID
	h.showDealCoverageMenu(chatID, msgID, dealID)
}

// Start adding a coverage: load participants into state, show payer keyboard
// "deal_cov_add" → 12 chars ✓ (dealID already in state)
func (h *Handler) handleDealCoverageAdd(ctx context.Context, tgID, chatID int64, msgID int) {
	st := h.getState(tgID)
	if st.dealID == "" {
		editText(h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	deal, err := h.client.GetDeal(ctx, st.dealID)
	if err != nil {
		editText(h.api, chatID, msgID, "Ошибка при загрузке сделки.", nil)
		return
	}
	participants, err := h.fetchUsers(ctx, deal.ParticipantIds)
	if err != nil {
		editText(h.api, chatID, msgID, "Ошибка при загрузке участников.", nil)
		return
	}
	for _, p := range participants {
		st.participantNames[p.Id] = p.Name
	}
	st.step = stepDealCovSelectPayer
	h.showDealCovPayerKeyboard(chatID, msgID, participants)
}

// Coverage payer selected → "deal_cov_payer:{payerID}" → 15+36=51 chars ✓
func (h *Handler) handleDealCoveragePayer(_ context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	payerID := strings.TrimPrefix(cb.Data, "deal_cov_payer:")
	st := h.getState(tgID)
	st.pendingCovPayerID = payerID
	st.step = stepDealCovSelectCovered
	h.showDealCovCoveredKeyboard(chatID, msgID, st)
}

// Covered person selected → "deal_cov_covered:{coveredID}" → 17+36=53 chars ✓
func (h *Handler) handleDealCoverageCovered(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	coveredID := strings.TrimPrefix(cb.Data, "deal_cov_covered:")
	st := h.getState(tgID)
	if st.dealID == "" {
		editText(h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.SetDealCoverage(ctx, st.dealID, st.pendingCovPayerID, coveredID); err != nil {
		editText(h.api, chatID, msgID, "Ошибка при сохранении покрытия.", nil)
		return
	}
	dealID := st.dealID
	st.step = stepIdle
	st.pendingCovPayerID = ""
	h.showDealCoverageMenu(chatID, msgID, dealID)
}

// Back to coverage menu (from payer selection)
func (h *Handler) handleDealCoverageBack(_ context.Context, tgID, chatID int64, msgID int) {
	st := h.getState(tgID)
	if st.dealID == "" {
		h.showMainMenu(chatID, msgID, "Главное меню:")
		return
	}
	st.step = stepIdle
	h.showDealCoverageMenu(chatID, msgID, st.dealID)
}

// Remove a coverage → "deal_cov_remove:{coveredID}" → 16+36=52 chars ✓
func (h *Handler) handleDealCoverageRemove(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	coveredID := strings.TrimPrefix(cb.Data, "deal_cov_remove:")
	st := h.getState(tgID)
	if st.dealID == "" {
		editText(h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.RemoveDealCoverage(ctx, st.dealID, coveredID); err != nil {
		editText(h.api, chatID, msgID, "Ошибка при удалении покрытия.", nil)
		return
	}
	h.showDealCoverageMenu(chatID, msgID, st.dealID)
}

// Payer selected → create purchase immediately
func (h *Handler) handleCreatePurchase(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	payerID := strings.TrimPrefix(cb.Data, "payer:")
	st := h.getState(tgID)
	if st.step != stepAwaitPurchasePayer {
		editText(h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	_, err := h.client.AddPurchase(ctx, st.dealID, st.purchaseTitle, st.purchaseAmt, payerID, "all", nil)
	if err != nil {
		editText(h.api, chatID, msgID, "Ошибка при добавлении покупки.", nil)
		return
	}
	dealID := st.dealID
	title := st.purchaseTitle
	h.resetState(tgID)
	editText(h.api, chatID, msgID, fmt.Sprintf("✅ Покупка «%s» добавлена!", title), nil)
	h.showDealMenu(chatID, 0, dealID)
}

func (h *Handler) handleBack(_ context.Context, tgID, chatID int64, msgID int) {
	st := h.getState(tgID)
	dealID := st.dealID
	switch st.step {
	case stepAwaitDealTitle:
		h.resetState(tgID)
		h.showMainMenu(chatID, msgID, "Главное меню:")
	case stepAwaitParticipantName, stepAwaitPurchaseTitle, stepAwaitPurchaseAmount, stepAwaitPurchasePayer:
		h.resetState(tgID)
		h.showDealMenu(chatID, msgID, dealID)
	default:
		h.resetState(tgID)
		h.showMainMenu(chatID, msgID, "Главное меню:")
	}
}
