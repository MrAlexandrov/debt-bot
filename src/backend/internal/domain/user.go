package domain

import "time"

type User struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type UserIdentity struct {
	ID         string
	UserID     string
	Platform   string
	ExternalID string
}
