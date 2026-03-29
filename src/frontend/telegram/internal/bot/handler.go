package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pb "github.com/mrralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
)

const platform = "telegram"

// FSM steps
const (
	stepIdle                  = ""
	stepAwaitDealTitle        = "await_deal_title"
	stepAwaitParticipantName  = "await_participant_name"
	stepAwaitPurchaseTitle    = "await_purchase_title"
	stepAwaitPurchaseAmount   = "await_purchase_amount"
	stepAwaitPurchasePayer    = "await_purchase_payer"
	stepDealCovSelectPayer    = "deal_cov_select_payer"
	stepDealCovSelectCovered  = "deal_cov_select_covered"
)

type userState struct {
	step              string
	dealID            string
	purchaseTitle     string
	purchaseAmt       int64
	purchasePayerID   string
	pendingCovPayerID string
	// cache: participant id → name
	participantNames map[string]string
}

type Handler struct {
	api    *tgbotapi.BotAPI
	client *Client
	mu     sync.Mutex
	states map[int64]*userState
}

func NewHandler(api *tgbotapi.BotAPI, client *Client) *Handler {
	return &Handler{
		api:    api,
		client: client,
		states: make(map[int64]*userState),
	}
}

func (h *Handler) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.api.GetUpdatesChan(u)
	for update := range updates {
		if update.CallbackQuery != nil {
			h.handleCallback(update.CallbackQuery)
		} else if update.Message != nil {
			h.handleMessage(update.Message)
		}
	}
	return nil
}

// --- State helpers ---

func (h *Handler) getState(userID int64) *userState {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.states[userID]; !ok {
		h.states[userID] = &userState{participantNames: make(map[string]string)}
	}
	return h.states[userID]
}

func (h *Handler) resetState(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.states[userID] = &userState{participantNames: make(map[string]string)}
}

// --- Message handler (text input) ---

func (h *Handler) handleMessage(msg *tgbotapi.Message) {
	ctx := context.Background()
	tgID := msg.From.ID

	if msg.IsCommand() && msg.Command() == "start" {
		h.resetState(tgID)
		user := h.resolveUser(ctx, msg.From)
		greeting := "Привет!"
		if user != nil {
			greeting = "Привет, " + user.Name + "!"
		}
		h.showMainMenu(msg.Chat.ID, 0, greeting+"\n\nЯ помогу рассчитать долги после совместных трат.")
		return
	}

	st := h.getState(tgID)
	text := strings.TrimSpace(msg.Text)

	switch st.step {
	case stepAwaitDealTitle:
		if text == "" {
			h.send(msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:")
			return
		}
		user := h.resolveUser(ctx, msg.From)
		if user == nil {
			return
		}
		deal, err := h.client.CreateDeal(ctx, text, user.Id)
		if err != nil {
			h.send(msg.Chat.ID, "Ошибка при создании сделки.")
			return
		}
		h.resetState(tgID)
		h.send(msg.Chat.ID, fmt.Sprintf("✅ Сделка «%s» создана!", deal.Title))
		h.showDealMenu(msg.Chat.ID, 0, deal.Id)

	case stepAwaitParticipantName:
		dealID := st.dealID
		newUser, notice, err := h.resolveParticipant(ctx, msg)
		if err != nil {
			h.send(msg.Chat.ID, "Ошибка при добавлении участника.")
			return
		}
		if newUser == nil {
			return
		}
		if _, err := h.client.AddDealParticipant(ctx, dealID, newUser.Id); err != nil {
			h.send(msg.Chat.ID, "Ошибка при добавлении в сделку.")
			return
		}
		h.resetState(tgID)
		h.send(msg.Chat.ID, fmt.Sprintf("✅ %s добавлен.%s", newUser.Name, notice))
		h.showDealMenu(msg.Chat.ID, 0, dealID)

	case stepAwaitPurchaseTitle:
		if text == "" {
			h.send(msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:")
			return
		}
		st.purchaseTitle = text
		st.step = stepAwaitPurchaseAmount
		h.send(msg.Chat.ID, "Введите сумму в рублях (например: 150 или 99.50 или 99,50):")

	case stepAwaitPurchaseAmount:
		amt, err := parseAmount(text)
		if err != nil || amt <= 0 {
			h.send(msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):")
			return
		}
		st.purchaseAmt = amt

		deal, err := h.client.GetDeal(ctx, st.dealID)
		if err != nil {
			h.send(msg.Chat.ID, "Ошибка при загрузке сделки.")
			return
		}
		participants, err := h.fetchUsers(ctx, deal.ParticipantIds)
		if err != nil || len(participants) == 0 {
			h.send(msg.Chat.ID, "Нет участников в сделке.")
			return
		}
		for _, p := range participants {
			st.participantNames[p.Id] = p.Name
		}
		st.step = stepAwaitPurchasePayer
		h.sendPayerKeyboard(msg.Chat.ID, participants)

	case stepAwaitPurchasePayer:
		h.send(msg.Chat.ID, "Выберите плательщика из кнопок выше.")

	case stepDealCovSelectPayer, stepDealCovSelectCovered:
		h.send(msg.Chat.ID, "Используйте кнопки для навигации.")
	}
}

// --- Callback handler (button presses) ---

func (h *Handler) handleCallback(cb *tgbotapi.CallbackQuery) {
	ctx := context.Background()
	tgID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	h.api.Request(tgbotapi.NewCallback(cb.ID, ""))

	data := cb.Data

	switch {
	case data == "main_menu":
		h.resetState(tgID)
		h.showMainMenu(chatID, msgID, "Главное меню:")

	case data == "new_deal":
		h.getState(tgID).step = stepAwaitDealTitle
		h.editText(chatID, msgID, "Введите название сделки:")

	case data == "my_deals":
		user := h.resolveUserFromCB(ctx, cb.From)
		if user == nil {
			return
		}
		h.showDealsList(chatID, msgID, user.Id)

	case strings.HasPrefix(data, "deal:"):
		dealID := strings.TrimPrefix(data, "deal:")
		h.resetState(tgID)
		h.showDealMenu(chatID, msgID, dealID)

	case strings.HasPrefix(data, "add_participant:"):
		dealID := strings.TrimPrefix(data, "add_participant:")
		st := h.getState(tgID)
		st.step = stepAwaitParticipantName
		st.dealID = dealID
		h.editText(chatID, msgID, "Добавьте участника одним из способов:\n\n• Введите имя\n• Отправьте @username\n• Перешлите сообщение от участника")

	case strings.HasPrefix(data, "add_purchase:"):
		dealID := strings.TrimPrefix(data, "add_purchase:")
		st := h.getState(tgID)
		st.step = stepAwaitPurchaseTitle
		st.dealID = dealID
		h.editText(chatID, msgID, "Введите название покупки:")

	case strings.HasPrefix(data, "purchases:"):
		dealID := strings.TrimPrefix(data, "purchases:")
		h.showPurchases(chatID, msgID, dealID)

	case strings.HasPrefix(data, "calculate:"):
		dealID := strings.TrimPrefix(data, "calculate:")
		h.showCalculation(chatID, msgID, dealID)

	// Deal-level coverages management screen
	// "deal_coverages:{dealID}" → max 15+36=51 chars ✓
	case strings.HasPrefix(data, "deal_coverages:"):
		dealID := strings.TrimPrefix(data, "deal_coverages:")
		st := h.getState(tgID)
		st.dealID = dealID
		h.showDealCoverageMenu(chatID, msgID, dealID)

	// Start adding a coverage: load participants into state, show payer keyboard
	// "deal_cov_add" → 12 chars ✓ (dealID already in state)
	case data == "deal_cov_add":
		st := h.getState(tgID)
		if st.dealID == "" {
			h.editText(chatID, msgID, "Сессия устарела. Начните заново.")
			return
		}
		deal, err := h.client.GetDeal(ctx, st.dealID)
		if err != nil {
			h.editText(chatID, msgID, "Ошибка при загрузке сделки.")
			return
		}
		participants, err := h.fetchUsers(ctx, deal.ParticipantIds)
		if err != nil {
			h.editText(chatID, msgID, "Ошибка при загрузке участников.")
			return
		}
		for _, p := range participants {
			st.participantNames[p.Id] = p.Name
		}
		st.step = stepDealCovSelectPayer
		h.showDealCovPayerKeyboard(chatID, msgID, participants)

	// Coverage payer selected → "deal_cov_payer:{payerID}" → 15+36=51 chars ✓
	case strings.HasPrefix(data, "deal_cov_payer:"):
		payerID := strings.TrimPrefix(data, "deal_cov_payer:")
		st := h.getState(tgID)
		st.pendingCovPayerID = payerID
		st.step = stepDealCovSelectCovered
		h.showDealCovCoveredKeyboard(chatID, msgID, st)

	// Covered person selected → "deal_cov_covered:{coveredID}" → 17+36=53 chars ✓
	case strings.HasPrefix(data, "deal_cov_covered:"):
		coveredID := strings.TrimPrefix(data, "deal_cov_covered:")
		st := h.getState(tgID)
		if st.dealID == "" {
			h.editText(chatID, msgID, "Сессия устарела. Начните заново.")
			return
		}
		if _, err := h.client.SetDealCoverage(ctx, st.dealID, st.pendingCovPayerID, coveredID); err != nil {
			h.editText(chatID, msgID, "Ошибка при сохранении покрытия.")
			return
		}
		dealID := st.dealID
		st.step = stepIdle
		st.pendingCovPayerID = ""
		h.showDealCoverageMenu(chatID, msgID, dealID)

	// Back to coverage menu (from payer selection)
	case data == "deal_cov_back":
		st := h.getState(tgID)
		if st.dealID == "" {
			h.showMainMenu(chatID, msgID, "Главное меню:")
			return
		}
		st.step = stepIdle
		h.showDealCoverageMenu(chatID, msgID, st.dealID)

	// Remove a coverage → "deal_cov_remove:{coveredID}" → 16+36=52 chars ✓
	case strings.HasPrefix(data, "deal_cov_remove:"):
		coveredID := strings.TrimPrefix(data, "deal_cov_remove:")
		st := h.getState(tgID)
		if st.dealID == "" {
			h.editText(chatID, msgID, "Сессия устарела. Начните заново.")
			return
		}
		if _, err := h.client.RemoveDealCoverage(ctx, st.dealID, coveredID); err != nil {
			h.editText(chatID, msgID, "Ошибка при удалении покрытия.")
			return
		}
		h.showDealCoverageMenu(chatID, msgID, st.dealID)

	// Payer selected → create purchase immediately
	case strings.HasPrefix(data, "payer:"):
		payerID := strings.TrimPrefix(data, "payer:")
		st := h.getState(tgID)
		if st.step != stepAwaitPurchasePayer {
			h.editText(chatID, msgID, "Сессия устарела. Начните заново.")
			return
		}
		_, err := h.client.AddPurchase(ctx, st.dealID, st.purchaseTitle, st.purchaseAmt, payerID, "all", nil)
		if err != nil {
			h.editText(chatID, msgID, "Ошибка при добавлении покупки.")
			return
		}
		dealID := st.dealID
		title := st.purchaseTitle
		h.resetState(tgID)
		h.editText(chatID, msgID, fmt.Sprintf("✅ Покупка «%s» добавлена!", title))
		h.showDealMenu(chatID, 0, dealID)
	}
}

// --- UI screens ---

func (h *Handler) showMainMenu(chatID int64, msgID int, text string) {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📦 Создать сделку", "new_deal"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои сделки", "my_deals"),
		),
	)
	h.sendOrEdit(chatID, msgID, text, &kb)
}

func (h *Handler) showDealsList(chatID int64, msgID int, userID string) {
	ctx := context.Background()
	deals, err := h.client.ListUserDeals(ctx, userID)
	if err != nil {
		h.editText(chatID, msgID, "Ошибка при загрузке сделок.")
		return
	}

	back := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "main_menu"))

	if len(deals) == 0 {
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📦 Создать сделку", "new_deal")),
			back,
		)
		h.sendOrEdit(chatID, msgID, "У вас пока нет сделок.", &kb)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, d := range deals {
		label := fmt.Sprintf("📦 %s (%d чел.)", d.Title, len(d.ParticipantIds))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "deal:"+d.Id),
		))
	}
	rows = append(rows, back)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.sendOrEdit(chatID, msgID, "Ваши сделки:", &kb)
}

func (h *Handler) showDealMenu(chatID int64, msgID int, dealID string) {
	ctx := context.Background()
	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		h.send(chatID, "Ошибка при загрузке сделки.")
		return
	}
	covCount := len(deal.Coverages)
	covLabel := "👥 Покрытие"
	if covCount > 0 {
		covLabel = fmt.Sprintf("👥 Покрытие (%d)", covCount)
	}
	text := fmt.Sprintf("📦 %s\nУчастников: %d", deal.Title, len(deal.ParticipantIds))
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Добавить участника", "add_participant:"+dealID),
			tgbotapi.NewInlineKeyboardButtonData("🛍 Добавить покупку", "add_purchase:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Покупки", "purchases:"+dealID),
			tgbotapi.NewInlineKeyboardButtonData("💰 Рассчитать", "calculate:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(covLabel, "deal_coverages:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← К сделкам", "my_deals"),
		),
	)
	h.sendOrEdit(chatID, msgID, text, &kb)
}

func (h *Handler) showDealCoverageMenu(chatID int64, msgID int, dealID string) {
	ctx := context.Background()
	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		h.editText(chatID, msgID, "Ошибка при загрузке сделки.")
		return
	}

	names := make(map[string]string)
	var sb strings.Builder
	sb.WriteString("👥 Покрытие расходов\n")
	sb.WriteString("(кто платит за кого во всех покупках сделки)\n")

	var rows [][]tgbotapi.InlineKeyboardButton

	if len(deal.Coverages) == 0 {
		sb.WriteString("\nПокрытий нет.")
	} else {
		sb.WriteString("\nТекущие покрытия:\n")
		for _, cov := range deal.Coverages {
			payer := h.resolveUserName(ctx, cov.PayerId, names)
			covered := h.resolveUserName(ctx, cov.CoveredId, names)
			sb.WriteString(fmt.Sprintf("• %s платит за %s\n", payer, covered))
			removeLabel := fmt.Sprintf("❌ %s→%s", payer, covered)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				// "deal_cov_remove:{coveredID}" → 16+36=52 chars ✓
				tgbotapi.NewInlineKeyboardButtonData(removeLabel, "deal_cov_remove:"+cov.CoveredId),
			))
		}
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить покрытие", "deal_cov_add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID),
		),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.sendOrEdit(chatID, msgID, sb.String(), &kb)
}

func (h *Handler) showDealCovPayerKeyboard(chatID int64, msgID int, participants []*pb.User) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			// "deal_cov_payer:{payerID}" → 15+36=51 chars ✓
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "deal_cov_payer:"+p.Id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal_cov_back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.sendOrEdit(chatID, msgID, "Кто платит за другого?", &kb)
}

func (h *Handler) showDealCovCoveredKeyboard(chatID int64, msgID int, st *userState) {
	payerName := st.participantNames[st.pendingCovPayerID]
	var rows [][]tgbotapi.InlineKeyboardButton
	for id, name := range st.participantNames {
		if id == st.pendingCovPayerID {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			// "deal_cov_covered:{coveredID}" → 17+36=53 chars ✓
			tgbotapi.NewInlineKeyboardButtonData(name, "deal_cov_covered:"+id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal_cov_add"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.sendOrEdit(chatID, msgID, fmt.Sprintf("За кого платит %s?", payerName), &kb)
}

func (h *Handler) showPurchases(chatID int64, msgID int, dealID string) {
	ctx := context.Background()
	purchases, err := h.client.ListDealPurchases(ctx, dealID)
	if err != nil {
		h.editText(chatID, msgID, "Ошибка при загрузке покупок.")
		return
	}

	back := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID)),
	)

	if len(purchases) == 0 {
		h.sendOrEdit(chatID, msgID, "Покупок пока нет.", &back)
		return
	}

	names := make(map[string]string)
	var sb strings.Builder
	sb.WriteString("Покупки:\n\n")
	var total int64
	for _, p := range purchases {
		payerName := h.resolveUserName(ctx, p.PaidBy, names)
		sb.WriteString(fmt.Sprintf("• %s — %s ₽ (платил %s)\n", p.Title, formatAmount(p.Amount), payerName))
		total += p.Amount
	}
	sb.WriteString(fmt.Sprintf("\nИтого: %s ₽", formatAmount(total)))
	h.sendOrEdit(chatID, msgID, sb.String(), &back)
}

func (h *Handler) showCalculation(chatID int64, msgID int, dealID string) {
	ctx := context.Background()
	result, err := h.client.CalculateDebts(ctx, dealID)
	if err != nil {
		h.editText(chatID, msgID, "Ошибка при расчёте.")
		return
	}

	back := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID)),
	)

	if len(result.Debts) == 0 {
		h.sendOrEdit(chatID, msgID, "✅ Все в расчёте, долгов нет!", &back)
		return
	}

	names := make(map[string]string)
	var sb strings.Builder
	sb.WriteString("Взаиморасчёты:\n\n")
	for _, d := range result.Debts {
		from := h.resolveUserName(ctx, d.FromUserId, names)
		to := h.resolveUserName(ctx, d.ToUserId, names)
		sb.WriteString(fmt.Sprintf("• %s → %s: %s ₽\n", from, to, formatAmount(d.Amount)))
	}
	h.sendOrEdit(chatID, msgID, sb.String(), &back)
}

func (h *Handler) sendPayerKeyboard(chatID int64, participants []*pb.User) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "payer:"+p.Id),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Кто оплатил?")
	msg.ReplyMarkup = kb
	h.api.Send(msg)
}

// --- Low-level send helpers ---

func (h *Handler) send(chatID int64, text string) {
	h.api.Send(tgbotapi.NewMessage(chatID, text))
}

func (h *Handler) editText(chatID int64, msgID int, text string) {
	h.api.Send(tgbotapi.NewEditMessageText(chatID, msgID, text))
}

func (h *Handler) sendOrEdit(chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	if msgID != 0 {
		edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		edit.ReplyMarkup = kb
		h.api.Send(edit)
	} else {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = kb
		h.api.Send(msg)
	}
}

// --- User helpers ---

func (h *Handler) resolveUser(ctx context.Context, from *tgbotapi.User) *pb.User {
	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	if name == "" {
		name = from.UserName
	}
	user, _, err := h.client.ResolveOrCreateUser(ctx, platform, strconv.FormatInt(from.ID, 10), name, from.UserName)
	if err != nil {
		return nil
	}
	return user
}

func (h *Handler) resolveUserFromCB(ctx context.Context, from *tgbotapi.User) *pb.User {
	return h.resolveUser(ctx, from)
}

// resolveParticipant determines how to add a participant based on the message:
//   - Forwarded message (ForwardFrom set)  → link by Telegram ID
//   - Privacy-protected forward (ForwardSenderName) → plain name
//   - @username text                        → link by username (merge on first bot use)
//   - Plain text                            → name only
//
// Returns the created/found user, an optional notice string, and an error.
func (h *Handler) resolveParticipant(ctx context.Context, msg *tgbotapi.Message) (*pb.User, string, error) {
	// Case 1: forwarded message with public sender info
	if msg.ForwardFrom != nil {
		from := msg.ForwardFrom
		name := strings.TrimSpace(from.FirstName + " " + from.LastName)
		if name == "" {
			name = from.UserName
		}
		user, _, err := h.client.ResolveOrCreateUser(ctx, platform, strconv.FormatInt(from.ID, 10), name, from.UserName)
		if err != nil {
			return nil, "", err
		}
		return user, " Telegram-аккаунт привязан.", nil
	}

	// Case 2: forwarded message but sender hid their identity
	if msg.ForwardSenderName != "" {
		user, err := h.client.CreateUser(ctx, msg.ForwardSenderName)
		if err != nil {
			return nil, "", err
		}
		return user, " (аккаунт скрыт настройками приватности)", nil
	}

	text := strings.TrimSpace(msg.Text)

	// Case 3: @username text
	if strings.HasPrefix(text, "@") {
		username := strings.TrimPrefix(text, "@")
		if username == "" {
			h.send(msg.Chat.ID, "Укажите username после @.")
			return nil, "", nil
		}
		user, _, err := h.client.ResolveOrCreateUser(ctx, "telegram_username", username, "@"+username, "")
		if err != nil {
			return nil, "", err
		}
		return user, " Когда откроет бота — аккаунты свяжутся автоматически.", nil
	}

	// Case 4: plain name
	if text == "" {
		h.send(msg.Chat.ID, "Введите имя, @username или перешлите сообщение.")
		return nil, "", nil
	}
	user, err := h.client.CreateUser(ctx, text)
	if err != nil {
		return nil, "", err
	}
	return user, "", nil
}

func (h *Handler) fetchUsers(ctx context.Context, ids []string) ([]*pb.User, error) {
	users := make([]*pb.User, 0, len(ids))
	for _, id := range ids {
		u, err := h.client.GetUser(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (h *Handler) resolveUserName(ctx context.Context, userID string, cache map[string]string) string {
	if name, ok := cache[userID]; ok {
		return name
	}
	u, err := h.client.GetUser(ctx, userID)
	if err != nil {
		return "?"
	}
	cache[userID] = u.Name
	return u.Name
}

// --- Amount helpers ---

// parseAmount parses a ruble amount like "150", "99.50", "99,50" into kopecks.
func parseAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.SplitN(s, ".", 2)

	rubles, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || rubles < 0 {
		return 0, fmt.Errorf("invalid amount")
	}

	kopecks := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		kStr := parts[1]
		switch len(kStr) {
		case 1:
			kStr += "0"
		default:
			kStr = kStr[:2]
		}
		kopecks, err = strconv.ParseInt(kStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid kopecks")
		}
	}
	return rubles*100 + kopecks, nil
}

// formatAmount converts kopecks to a human-readable ruble string.
func formatAmount(kopecks int64) string {
	r := kopecks / 100
	k := kopecks % 100
	if k == 0 {
		return fmt.Sprintf("%d", r)
	}
	return fmt.Sprintf("%d.%02d", r, k)
}
