package domain

type DebtItem struct {
	FromUserID string
	ToUserID   string
	Amount     int64 // in kopecks
}

type CalculationResult struct {
	Debts    []DebtItem
	Balances map[string]int64 // user_id -> balance in kopecks
}
