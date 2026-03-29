package service

import (
	"testing"

	"github.com/mrralexandrov/debt-bot/backend/internal/domain"
)

func TestCalculateBalances_ExampleFromSpec(t *testing.T) {
	// Example from plan: 6 participants, total 21300 kopecks
	// Maxim paid 12500, Nikita paid 8300, Lyosha paid 500, others paid 0
	participants := []string{"maxim", "nikita", "lyosha", "nastya", "tanya", "angelina"}
	purchases := []*domain.Purchase{
		{PaidBy: "maxim", Amount: 12500, SplitMode: domain.SplitModeAll},
		{PaidBy: "nikita", Amount: 8300, SplitMode: domain.SplitModeAll},
		{PaidBy: "lyosha", Amount: 500, SplitMode: domain.SplitModeAll},
	}

	balances := calculateBalances(purchases, participants, nil)

	// Per-purchase integer division (remainder stays with payer):
	// 12500/6=2083 rem2, 8300/6=1383 rem2, 500/6=83 rem2
	// Each non-payer share: 2083+1383+83 = 3549
	// Maxim:    -2083 + 12500 - 1383 - 83 = +8951
	// Nikita:   -2083 - 1383 +  8300 - 83 = +4751
	// Lyosha:   -2083 - 1383 -   83 +  500 = -3049
	// Nastya/Tanya/Angelina: -(2083+1383+83) = -3549 each
	expected := map[string]int64{
		"maxim":    8951,
		"nikita":   4751,
		"lyosha":   -3049,
		"nastya":   -3549,
		"tanya":    -3549,
		"angelina": -3549,
	}

	for uid, want := range expected {
		if got := balances[uid]; got != want {
			t.Errorf("balance[%s] = %d, want %d", uid, got, want)
		}
	}
}

func TestMinimizeTransactions_TwoParties(t *testing.T) {
	balances := map[string]int64{
		"alice": 1000,
		"bob":   -1000,
	}
	debts := minimizeTransactions(balances)
	if len(debts) != 1 {
		t.Fatalf("expected 1 debt, got %d", len(debts))
	}
	if debts[0].FromUserID != "bob" || debts[0].ToUserID != "alice" || debts[0].Amount != 1000 {
		t.Errorf("unexpected debt: %+v", debts[0])
	}
}

func TestMinimizeTransactions_ThreeParties(t *testing.T) {
	// alice +300, bob +200, carol -500
	balances := map[string]int64{
		"alice": 300,
		"bob":   200,
		"carol": -500,
	}
	debts := minimizeTransactions(balances)
	// carol should pay 300 to alice and 200 to bob (or equivalent minimized)
	total := int64(0)
	for _, d := range debts {
		total += d.Amount
	}
	if total != 500 {
		t.Errorf("total debt transfers = %d, want 500", total)
	}
}

func TestCalculateBalances_CustomSplit(t *testing.T) {
	participants := []string{"alice", "bob", "carol"}
	purchases := []*domain.Purchase{
		{
			PaidBy:         "alice",
			Amount:         200,
			SplitMode:      domain.SplitModeCustom,
			ParticipantIDs: []string{"alice", "bob"}, // carol is excluded
		},
	}
	balances := calculateBalances(purchases, participants, nil)
	// alice and bob each owe 100, alice paid 200 -> alice net +100
	// carol owes nothing for this purchase
	if balances["alice"] != 100 {
		t.Errorf("alice balance = %d, want 100", balances["alice"])
	}
	if balances["bob"] != -100 {
		t.Errorf("bob balance = %d, want -100", balances["bob"])
	}
	if balances["carol"] != 0 {
		t.Errorf("carol balance = %d, want 0", balances["carol"])
	}
}
