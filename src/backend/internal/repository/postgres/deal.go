package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mralexandrov/debt-bot/backend/internal/domain"
)

type DealRepository struct {
	db *pgxpool.Pool
}

func NewDealRepository(db *pgxpool.Pool) *DealRepository {
	return &DealRepository{db: db}
}

func (r *DealRepository) Create(ctx context.Context, title, createdBy string) (*domain.Deal, error) {
	var d domain.Deal
	err := r.db.QueryRow(ctx,
		`INSERT INTO deals (title, created_by) VALUES ($1, $2) RETURNING id, title, created_by, created_at`,
		title, createdBy,
	).Scan(&d.ID, &d.Title, &d.CreatedBy, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create deal: %w", err)
	}
	return &d, nil
}

func (r *DealRepository) GetByID(ctx context.Context, id string) (*domain.Deal, error) {
	var d domain.Deal
	err := r.db.QueryRow(ctx,
		`SELECT id, title, created_by, created_at FROM deals WHERE id = $1`,
		id,
	).Scan(&d.ID, &d.Title, &d.CreatedBy, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("deal %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get deal: %w", err)
	}

	participants, err := r.GetParticipants(ctx, id)
	if err != nil {
		return nil, err
	}
	d.ParticipantIDs = participants

	coverages, err := r.GetCoverages(ctx, id)
	if err != nil {
		return nil, err
	}
	d.Coverages = coverages
	return &d, nil
}

func (r *DealRepository) SetCoverage(ctx context.Context, dealID, payerID, coveredID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO deal_coverage (deal_id, payer_id, covered_id) VALUES ($1, $2, $3)
		 ON CONFLICT (deal_id, covered_id) DO UPDATE SET payer_id = EXCLUDED.payer_id`,
		dealID, payerID, coveredID,
	)
	if err != nil {
		return fmt.Errorf("set coverage: %w", err)
	}
	return nil
}

func (r *DealRepository) RemoveCoverage(ctx context.Context, dealID, coveredID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM deal_coverage WHERE deal_id = $1 AND covered_id = $2`,
		dealID, coveredID,
	)
	if err != nil {
		return fmt.Errorf("remove coverage: %w", err)
	}
	return nil
}

func (r *DealRepository) GetCoverages(ctx context.Context, dealID string) ([]domain.Coverage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT payer_id, covered_id FROM deal_coverage WHERE deal_id = $1`,
		dealID,
	)
	if err != nil {
		return nil, fmt.Errorf("get coverages: %w", err)
	}
	defer rows.Close()

	var coverages []domain.Coverage
	for rows.Next() {
		var c domain.Coverage
		if err := rows.Scan(&c.PayerID, &c.CoveredID); err != nil {
			return nil, fmt.Errorf("scan coverage: %w", err)
		}
		coverages = append(coverages, c)
	}
	return coverages, rows.Err()
}

func (r *DealRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Deal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT d.id, d.title, d.created_by, d.created_at
		 FROM deals d
		 JOIN deal_participants dp ON dp.deal_id = d.id
		 WHERE dp.user_id = $1
		 ORDER BY d.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list deals: %w", err)
	}
	defer rows.Close()

	var deals []*domain.Deal
	for rows.Next() {
		var d domain.Deal
		if err := rows.Scan(&d.ID, &d.Title, &d.CreatedBy, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan deal: %w", err)
		}
		deals = append(deals, &d)
	}
	return deals, rows.Err()
}

func (r *DealRepository) AddParticipant(ctx context.Context, dealID, userID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO deal_participants (deal_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		dealID, userID,
	)
	if err != nil {
		return fmt.Errorf("add deal participant: %w", err)
	}
	return nil
}

func (r *DealRepository) GetParticipants(ctx context.Context, dealID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT user_id FROM deal_participants WHERE deal_id = $1`,
		dealID,
	)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
