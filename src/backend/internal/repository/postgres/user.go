package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mralexandrov/debt-bot/backend/internal/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, name string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (name) VALUES ($1) RETURNING id, name, created_at`,
		name,
	).Scan(&u.ID, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, name, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, id, name string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`UPDATE users SET name = $1 WHERE id = $2 RETURNING id, name, created_at`,
		name, id,
	).Scan(&u.ID, &u.Name, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindIdentity(ctx context.Context, platform, externalID string) (*domain.UserIdentity, error) {
	var i domain.UserIdentity
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, platform, external_id FROM user_identities WHERE platform = $1 AND external_id = $2`,
		platform, externalID,
	).Scan(&i.ID, &i.UserID, &i.Platform, &i.ExternalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find identity: %w", err)
	}
	return &i, nil
}

func (r *UserRepository) CreateIdentity(ctx context.Context, userID, platform, externalID string) (*domain.UserIdentity, error) {
	var i domain.UserIdentity
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_identities (user_id, platform, external_id) VALUES ($1, $2, $3) RETURNING id, user_id, platform, external_id`,
		userID, platform, externalID,
	).Scan(&i.ID, &i.UserID, &i.Platform, &i.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	return &i, nil
}
