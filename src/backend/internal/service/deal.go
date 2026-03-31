package service

import (
	"context"
	"fmt"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"github.com/mralexandrov/debt-bot/backend/internal/repository"
	"go.opentelemetry.io/otel/attribute"
)

type DealService struct {
	deals     repository.DealRepository
	purchases repository.PurchaseRepository
}

func NewDealService(deals repository.DealRepository, purchases repository.PurchaseRepository) *DealService {
	return &DealService{deals: deals, purchases: purchases}
}

func (s *DealService) Create(ctx context.Context, title, createdBy string) (*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.title", title),
		attribute.String("deal.created_by", createdBy),
	)

	deal, err := s.deals.Create(ctx, title, createdBy)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.String("deal.id", deal.ID))

	// Creator is automatically a participant
	if err := s.deals.AddParticipant(ctx, deal.ID, createdBy); err != nil {
		return nil, fmt.Errorf("add creator as participant: %w", err)
	}
	deal.ParticipantIDs = []string{createdBy}
	return deal, nil
}

func (s *DealService) GetByID(ctx context.Context, id string) (*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.GetByID")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", id))
	return s.deals.GetByID(ctx, id)
}

func (s *DealService) ListByUserID(ctx context.Context, userID string) ([]*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.ListByUserID")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", userID))

	deals, err := s.deals.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("deal.count", len(deals)))
	return deals, nil
}

func (s *DealService) AddParticipant(ctx context.Context, dealID, userID string) (*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.AddParticipant")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("user.id", userID),
	)

	if err := s.deals.AddParticipant(ctx, dealID, userID); err != nil {
		return nil, err
	}
	return s.deals.GetByID(ctx, dealID)
}

func (s *DealService) AddPurchase(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, participantIDs []string) (*domain.Purchase, error) {
	ctx, span := tracer.Start(ctx, "DealService.AddPurchase")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("purchase.title", title),
		attribute.Int64("purchase.amount", amount),
		attribute.String("purchase.split_mode", splitMode),
		attribute.String("purchase.paid_by", paidBy),
	)

	purchase, err := s.purchases.Create(ctx, dealID, title, amount, paidBy, splitMode)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.String("purchase.id", purchase.ID))

	if splitMode == domain.SplitModeCustom {
		for _, uid := range participantIDs {
			if err := s.purchases.AddParticipant(ctx, purchase.ID, uid); err != nil {
				return nil, fmt.Errorf("add purchase participant: %w", err)
			}
		}
		purchase.ParticipantIDs = participantIDs
	}

	return purchase, nil
}

func (s *DealService) SetCoverage(ctx context.Context, dealID, payerID, coveredID string) (*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.SetCoverage")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("coverage.payer_id", payerID),
		attribute.String("coverage.covered_id", coveredID),
	)

	if err := s.deals.SetCoverage(ctx, dealID, payerID, coveredID); err != nil {
		return nil, err
	}
	return s.deals.GetByID(ctx, dealID)
}

func (s *DealService) RemoveCoverage(ctx context.Context, dealID, coveredID string) (*domain.Deal, error) {
	ctx, span := tracer.Start(ctx, "DealService.RemoveCoverage")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("coverage.covered_id", coveredID),
	)

	if err := s.deals.RemoveCoverage(ctx, dealID, coveredID); err != nil {
		return nil, err
	}
	return s.deals.GetByID(ctx, dealID)
}

func (s *DealService) ListPurchases(ctx context.Context, dealID string) ([]*domain.Purchase, error) {
	ctx, span := tracer.Start(ctx, "DealService.ListPurchases")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", dealID))

	purchases, err := s.purchases.ListByDealID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("purchase.count", len(purchases)))
	return purchases, nil
}
