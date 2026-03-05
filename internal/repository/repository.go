package repository

import (
	"backend/internal/domain"
	"context"
	"database/sql"
	"errors"
)

var ErrNotFound = errors.New("not found")

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error
}

type UserRepository interface {
	GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id int64) (domain.User, error)
	UpdateBalance(ctx context.Context, tx *sql.Tx, userID int64, newBalance int64) error
}

type WithdrawalRepository interface {
	GetByUserAndIdempotencyKeyForUpdate(ctx context.Context, tx *sql.Tx, userID int64, key string) (domain.Withdrawal, error)
	Create(ctx context.Context, tx *sql.Tx, withdrawal domain.Withdrawal) (domain.Withdrawal, error)
	GetByID(ctx context.Context, id int64) (domain.Withdrawal, error)
	CountByUserID(ctx context.Context, userID int64) (int64, error)
}
