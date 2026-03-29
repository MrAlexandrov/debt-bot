package domain

const (
	SplitModeAll    = "all"
	SplitModeCustom = "custom"
)

// Coverage means PayerID covers CoveredID's share in a purchase.
type Coverage struct {
	PayerID   string
	CoveredID string
}

type Purchase struct {
	ID             string
	DealID         string
	Title          string
	Amount         int64 // in kopecks
	PaidBy         string
	SplitMode      string
	ParticipantIDs []string
}
