package service

import "errors"

var (
	ErrInvalidAmount         = errors.New("amount must be greater than zero")
	ErrInvalidCurrency       = errors.New("currency must be USDT")
	ErrUserNotFound          = errors.New("user not found")
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrIdempotencyConflict   = errors.New("idempotency key already used with different payload")
	ErrInvalidIdempotencyKey = errors.New("idempotency_key is required")
	ErrInvalidDestination    = errors.New("destination is required")
	ErrWithdrawalNotFound    = errors.New("withdrawal not found")
	ErrInvalidStatus         = errors.New("invalid withdrawal status")
)
