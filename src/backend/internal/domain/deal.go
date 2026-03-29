package domain

import "time"

type Deal struct {
	ID             string
	Title          string
	CreatedBy      string
	CreatedAt      time.Time
	ParticipantIDs []string
	Coverages      []Coverage
}
