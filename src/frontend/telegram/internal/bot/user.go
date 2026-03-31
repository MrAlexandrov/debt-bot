package bot

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
)

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
	if after, ok := strings.CutPrefix(text, "@"); ok {
		username := after
		if username == "" {
			send(h.api, msg.Chat.ID, "Укажите username после @.", nil)
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
		send(h.api, msg.Chat.ID, "Введите имя, @username или перешлите сообщение.", nil)
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
