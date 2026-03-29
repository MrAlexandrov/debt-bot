package service

import (
	"context"
	"sort"

	"github.com/mrralexandrov/debt-bot/backend/internal/domain"
	"github.com/mrralexandrov/debt-bot/backend/internal/repository"
)

type DebtService struct {
	deals     repository.DealRepository
	purchases repository.PurchaseRepository
}

func NewDebtService(deals repository.DealRepository, purchases repository.PurchaseRepository) *DebtService {
	return &DebtService{deals: deals, purchases: purchases}
}

func (s *DebtService) Calculate(ctx context.Context, dealID string) (*domain.CalculationResult, error) {
	participants, err := s.deals.GetParticipants(ctx, dealID)
	if err != nil {
		return nil, err
	}

	purchases, err := s.purchases.ListByDealID(ctx, dealID)
	if err != nil {
		return nil, err
	}

	coverages, err := s.deals.GetCoverages(ctx, dealID)
	if err != nil {
		return nil, err
	}

	balances := calculateBalances(purchases, participants, coverages)
	debts := minimizeTransactions(balances)

	return &domain.CalculationResult{
		Debts:    debts,
		Balances: balances,
	}, nil
}

// calculateBalances computes net balance for each participant across all purchases.
// Positive balance = owed money (creditor), negative = owes money (debtor).
func calculateBalances(purchases []*domain.Purchase, dealParticipants []string, coverages []domain.Coverage) map[string]int64 {
	balances := make(map[string]int64)
	for _, id := range dealParticipants {
		balances[id] = 0
	}

	// Build coverage map once for the whole deal: covered → who covers their share
	covers := make(map[string]string, len(coverages))
	for _, cov := range coverages {
		covers[cov.CoveredID] = cov.PayerID
	}

	for _, p := range purchases {
		var splitAmong []string
		if p.SplitMode == domain.SplitModeCustom {
			splitAmong = p.ParticipantIDs
		} else {
			splitAmong = dealParticipants
		}

		n := int64(len(splitAmong))
		if n == 0 {
			continue
		}

		share := p.Amount / n

		// Charge each participant's share — to themselves or to whoever covers them
		for _, uid := range splitAmong {
			if covererID, ok := covers[uid]; ok {
				balances[covererID] -= share
			} else {
				balances[uid] -= share
			}
		}
		balances[p.PaidBy] += p.Amount
	}

	return balances
}

type balanceEntry struct {
	userID  string
	balance int64
}

// minimizeTransactions uses a greedy algorithm to minimize the number of transactions.
func minimizeTransactions(balances map[string]int64) []domain.DebtItem {
	var entries []balanceEntry
	for uid, bal := range balances {
		if bal != 0 {
			entries = append(entries, balanceEntry{uid, bal})
		}
	}

	// Sort descending: creditors first, then debtors
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].balance > entries[j].balance
	})

	var debts []domain.DebtItem
	i, j := 0, len(entries)-1

	for i < j {
		creditor := &entries[i]
		debtor := &entries[j]

		if creditor.balance <= 0 || debtor.balance >= 0 {
			break
		}

		amount := min(creditor.balance, -debtor.balance)

		debts = append(debts, domain.DebtItem{
			FromUserID: debtor.userID,
			ToUserID:   creditor.userID,
			Amount:     amount,
		})

		creditor.balance -= amount
		debtor.balance += amount

		if creditor.balance == 0 {
			i++
		}
		if debtor.balance == 0 {
			j--
		}
	}

	return debts
}
