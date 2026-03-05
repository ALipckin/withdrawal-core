package domain

import "time"

type LedgerEntry struct {
	ID           int64
	WithdrawalID int64
	UserID       int64
	EntryType    string
	AmountDelta  int64
	CreatedAt    time.Time
}
