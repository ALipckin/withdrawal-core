package domain

import "time"

type Withdrawal struct {
	ID             int64
	UserID         int64
	Amount         int64
	Currency       string
	Destination    string
	IdempotencyKey string
	PayloadHash    string
	Status         string
	CreatedAt      time.Time
	ConfirmedAt    *time.Time
}
