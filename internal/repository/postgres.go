package repository

import (
	"backend/internal/domain"
	"context"
	"database/sql"
	"fmt"
)

type PostgresTxManager struct {
	db *sql.DB
}

func NewPostgresTxManager(db *sql.DB) *PostgresTxManager {
	return &PostgresTxManager{db: db}
}

func (m *PostgresTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err = fn(ctx, tx); err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rollback tx: %v, cause: %w", rollbackErr, err)
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

type PostgresUserRepository struct{}

func NewPostgresUserRepository() *PostgresUserRepository {
	return &PostgresUserRepository{}
}

func (r *PostgresUserRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id int64) (domain.User, error) {
	const query = `SELECT id, balance FROM users WHERE id = $1 FOR UPDATE`

	var user domain.User
	if err := tx.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Balance); err != nil {
		if err == sql.ErrNoRows {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user for update: %w", err)
	}

	return user, nil
}

func (r *PostgresUserRepository) UpdateBalance(ctx context.Context, tx *sql.Tx, userID int64, newBalance int64) error {
	const query = `UPDATE users SET balance = $1 WHERE id = $2`

	result, err := tx.ExecContext(ctx, query, newBalance, userID)
	if err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

type PostgresWithdrawalRepository struct {
	db *sql.DB
}

func NewPostgresWithdrawalRepository(db *sql.DB) *PostgresWithdrawalRepository {
	return &PostgresWithdrawalRepository{db: db}
}

func (r *PostgresWithdrawalRepository) GetByUserAndIdempotencyKeyForUpdate(ctx context.Context, tx *sql.Tx, userID int64, key string) (domain.Withdrawal, error) {
	const query = `
		SELECT id, user_id, amount, currency, destination, idempotency_key, payload_hash, status, created_at, confirmed_at
		FROM withdrawals
		WHERE user_id = $1 AND idempotency_key = $2
		FOR UPDATE`

	row := tx.QueryRowContext(ctx, query, userID, key)
	return scanWithdrawal(row)
}

func (r *PostgresWithdrawalRepository) Create(ctx context.Context, tx *sql.Tx, withdrawal domain.Withdrawal) (domain.Withdrawal, error) {
	const query = `
		INSERT INTO withdrawals (user_id, amount, currency, destination, idempotency_key, payload_hash, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, amount, currency, destination, idempotency_key, payload_hash, status, created_at, confirmed_at`

	row := tx.QueryRowContext(
		ctx,
		query,
		withdrawal.UserID,
		withdrawal.Amount,
		withdrawal.Currency,
		withdrawal.Destination,
		withdrawal.IdempotencyKey,
		withdrawal.PayloadHash,
		withdrawal.Status,
	)
	return scanWithdrawal(row)
}

func (r *PostgresWithdrawalRepository) GetByID(ctx context.Context, id int64) (domain.Withdrawal, error) {
	const query = `
		SELECT id, user_id, amount, currency, destination, idempotency_key, payload_hash, status, created_at, confirmed_at
		FROM withdrawals
		WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanWithdrawal(row)
}

func (r *PostgresWithdrawalRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id int64) (domain.Withdrawal, error) {
	const query = `
		SELECT id, user_id, amount, currency, destination, idempotency_key, payload_hash, status, created_at, confirmed_at
		FROM withdrawals
		WHERE id = $1
		FOR UPDATE`

	row := tx.QueryRowContext(ctx, query, id)
	return scanWithdrawal(row)
}

func (r *PostgresWithdrawalRepository) MarkConfirmed(ctx context.Context, tx *sql.Tx, id int64) (domain.Withdrawal, error) {
	const query = `
		UPDATE withdrawals
		SET status = 'confirmed', confirmed_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, amount, currency, destination, idempotency_key, payload_hash, status, created_at, confirmed_at`

	row := tx.QueryRowContext(ctx, query, id)
	return scanWithdrawal(row)
}

func (r *PostgresWithdrawalRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count withdrawals by user id: %w", err)
	}

	return count, nil
}

type PostgresLedgerRepository struct{}

func NewPostgresLedgerRepository() *PostgresLedgerRepository {
	return &PostgresLedgerRepository{}
}

func (r *PostgresLedgerRepository) Create(ctx context.Context, tx *sql.Tx, entry domain.LedgerEntry) error {
	const query = `
		INSERT INTO ledger_entries (withdrawal_id, user_id, entry_type, amount_delta)
		VALUES ($1, $2, $3, $4)`

	_, err := tx.ExecContext(ctx, query, entry.WithdrawalID, entry.UserID, entry.EntryType, entry.AmountDelta)
	if err != nil {
		return fmt.Errorf("create ledger entry: %w", err)
	}

	return nil
}

func scanWithdrawal(row *sql.Row) (domain.Withdrawal, error) {
	var withdrawal domain.Withdrawal
	var confirmedAt sql.NullTime

	if err := row.Scan(
		&withdrawal.ID,
		&withdrawal.UserID,
		&withdrawal.Amount,
		&withdrawal.Currency,
		&withdrawal.Destination,
		&withdrawal.IdempotencyKey,
		&withdrawal.PayloadHash,
		&withdrawal.Status,
		&withdrawal.CreatedAt,
		&confirmedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return domain.Withdrawal{}, ErrNotFound
		}
		return domain.Withdrawal{}, fmt.Errorf("scan withdrawal: %w", err)
	}

	if confirmedAt.Valid {
		timestamp := confirmedAt.Time
		withdrawal.ConfirmedAt = &timestamp
	}

	return withdrawal, nil
}
