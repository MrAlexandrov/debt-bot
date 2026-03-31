package domain

import "errors"

// Sentinel errors for participant removal constraints.
// Use errors.Is to check these in upper layers.
var (
	ErrParticipantHasPurchases    = errors.New("participant has purchases in this deal")
	ErrParticipantIsCoveragePayer = errors.New("participant is a coverage payer in this deal")
	ErrParticipantIsCovered       = errors.New("participant is covered by another participant in this deal")
)
