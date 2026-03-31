package service

import (
	"context"
	"fmt"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"github.com/mralexandrov/debt-bot/backend/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const platformTelegramUsername = "telegram_username"

var tracer = otel.Tracer("debt-bot-backend/service")

type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) Create(ctx context.Context, name string) (*domain.User, error) {
	ctx, span := tracer.Start(ctx, "UserService.Create")
	defer span.End()

	user, err := s.users.Create(ctx, name)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.String("user.id", user.ID))
	return user, nil
}

// ResolveOrCreate finds or creates a user by platform identity.
//
// username is the Telegram @username (without @), used only when platform="telegram".
// If provided and no user is found by Telegram ID, the service checks for an existing
// "telegram_username" identity and merges it with the Telegram ID (auto-linking).
func (s *UserService) ResolveOrCreate(ctx context.Context, platform, externalID, name, username string) (*domain.User, bool, error) {
	ctx, span := tracer.Start(ctx, "UserService.ResolveOrCreate")
	defer span.End()
	span.SetAttributes(
		attribute.String("user.platform", platform),
		attribute.String("user.external_id", externalID),
	)

	// 1. Look up by primary identity (Telegram ID or telegram_username)
	identity, err := s.users.FindIdentity(ctx, platform, externalID)
	if err != nil {
		return nil, false, fmt.Errorf("find identity: %w", err)
	}
	if identity != nil {
		span.SetAttributes(attribute.String("user.resolve_result", "found"))
		user, err := s.users.GetByID(ctx, identity.UserID)
		if err == nil {
			span.SetAttributes(attribute.String("user.id", user.ID))
		}
		return user, false, err
	}

	// 2. For real Telegram users: try to merge with an existing telegram_username identity
	if platform == "telegram" && username != "" {
		usernameIdentity, err := s.users.FindIdentity(ctx, platformTelegramUsername, username)
		if err != nil {
			return nil, false, fmt.Errorf("find username identity: %w", err)
		}
		if usernameIdentity != nil {
			// Link this Telegram ID to the already-existing user
			if _, err := s.users.CreateIdentity(ctx, usernameIdentity.UserID, platform, externalID); err != nil {
				return nil, false, fmt.Errorf("link telegram identity: %w", err)
			}
			span.SetAttributes(
				attribute.String("user.resolve_result", "linked"),
				attribute.String("user.id", usernameIdentity.UserID),
			)
			user, err := s.users.GetByID(ctx, usernameIdentity.UserID)
			return user, false, err
		}
	}

	// 3. Create new user
	user, err := s.users.Create(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("create user: %w", err)
	}
	if _, err := s.users.CreateIdentity(ctx, user.ID, platform, externalID); err != nil {
		return nil, false, fmt.Errorf("create identity: %w", err)
	}
	// Also store telegram_username identity so future merges work
	if platform == "telegram" && username != "" {
		_, _ = s.users.CreateIdentity(ctx, user.ID, platformTelegramUsername, username)
	}
	span.SetAttributes(
		attribute.String("user.resolve_result", "created"),
		attribute.String("user.id", user.ID),
	)
	return user, true, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	ctx, span := tracer.Start(ctx, "UserService.GetByID")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id))
	return s.users.GetByID(ctx, id)
}

func (s *UserService) Update(ctx context.Context, id, name string) (*domain.User, error) {
	ctx, span := tracer.Start(ctx, "UserService.Update")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id))
	return s.users.Update(ctx, id, name)
}
