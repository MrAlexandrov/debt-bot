package service

import (
	"context"
	"fmt"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"github.com/mralexandrov/debt-bot/backend/internal/repository"
)

const platformTelegramUsername = "telegram_username"

type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) Create(ctx context.Context, name string) (*domain.User, error) {
	return s.users.Create(ctx, name)
}

// ResolveOrCreate finds or creates a user by platform identity.
//
// username is the Telegram @username (without @), used only when platform="telegram".
// If provided and no user is found by Telegram ID, the service checks for an existing
// "telegram_username" identity and merges it with the Telegram ID (auto-linking).
func (s *UserService) ResolveOrCreate(ctx context.Context, platform, externalID, name, username string) (*domain.User, bool, error) {
	// 1. Look up by primary identity (Telegram ID or telegram_username)
	identity, err := s.users.FindIdentity(ctx, platform, externalID)
	if err != nil {
		return nil, false, fmt.Errorf("find identity: %w", err)
	}
	if identity != nil {
		user, err := s.users.GetByID(ctx, identity.UserID)
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
	return user, true, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *UserService) Update(ctx context.Context, id, name string) (*domain.User, error) {
	return s.users.Update(ctx, id, name)
}
