package service

import (
	"backend/internal/domain"
	"backend/internal/repository"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	CurrencyUSDT        = "USDT"
	WithdrawalPending   = "pending"
	WithdrawalConfirmed = "confirmed"
	LedgerCreateType    = "withdrawal_create"
	LedgerConfirmType   = "withdrawal_confirm"
)

type EventLogger interface {
	Info(ctx context.Context, msg string, attrs ...any)
	Error(ctx context.Context, msg string, attrs ...any)
}

type CreateWithdrawalInput struct {
	UserID         int64
	Amount         int64
	Currency       string
	Destination    string
	IdempotencyKey string
}

type WithdrawalService struct {
	txManager            repository.TxManager
	userRepository       repository.UserRepository
	withdrawalRepository repository.WithdrawalRepository
	ledgerRepository     repository.LedgerRepository
	logger               EventLogger
}

func NewWithdrawalService(
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	withdrawalRepository repository.WithdrawalRepository,
	ledgerRepository repository.LedgerRepository,
	logger EventLogger,
) *WithdrawalService {
	if logger == nil {
		logger = noopLogger{}
	}

	return &WithdrawalService{
		txManager:            txManager,
		userRepository:       userRepository,
		withdrawalRepository: withdrawalRepository,
		ledgerRepository:     ledgerRepository,
		logger:               logger,
	}
}

func (s *WithdrawalService) CreateWithdrawal(ctx context.Context, input CreateWithdrawalInput) (domain.Withdrawal, bool, error) {
	if input.Amount <= 0 {
		s.logCreateFailure(ctx, input, ErrInvalidAmount)
		return domain.Withdrawal{}, false, ErrInvalidAmount
	}
	if input.Currency != CurrencyUSDT {
		s.logCreateFailure(ctx, input, ErrInvalidCurrency)
		return domain.Withdrawal{}, false, ErrInvalidCurrency
	}
	if input.IdempotencyKey == "" {
		s.logCreateFailure(ctx, input, ErrInvalidIdempotencyKey)
		return domain.Withdrawal{}, false, ErrInvalidIdempotencyKey
	}
	if input.Destination == "" {
		s.logCreateFailure(ctx, input, ErrInvalidDestination)
		return domain.Withdrawal{}, false, ErrInvalidDestination
	}

	payloadHash := buildPayloadHash(input)

	var out domain.Withdrawal
	var isReplay bool
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		user, err := s.userRepository.GetByIDForUpdate(ctx, tx, input.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrUserNotFound
			}
			return err
		}

		existing, err := s.withdrawalRepository.GetByUserAndIdempotencyKeyForUpdate(ctx, tx, input.UserID, input.IdempotencyKey)
		if err == nil {
			if existing.PayloadHash != payloadHash {
				return ErrIdempotencyConflict
			}
			out = existing
			isReplay = true
			return nil
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		if user.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		newBalance := user.Balance - input.Amount
		if err = s.userRepository.UpdateBalance(ctx, tx, user.ID, newBalance); err != nil {
			return err
		}

		toCreate := domain.Withdrawal{
			UserID:         input.UserID,
			Amount:         input.Amount,
			Currency:       input.Currency,
			Destination:    input.Destination,
			IdempotencyKey: input.IdempotencyKey,
			PayloadHash:    payloadHash,
			Status:         WithdrawalPending,
		}

		created, err := s.withdrawalRepository.Create(ctx, tx, toCreate)
		if err != nil {
			return err
		}

		if err = s.ledgerRepository.Create(ctx, tx, domain.LedgerEntry{
			WithdrawalID: created.ID,
			UserID:       created.UserID,
			EntryType:    LedgerCreateType,
			AmountDelta:  -created.Amount,
		}); err != nil {
			return err
		}

		out = created
		return nil
	})
	if err != nil {
		s.logCreateFailure(ctx, input, err)
		return domain.Withdrawal{}, false, err
	}

	s.logCreateSuccess(ctx, out, isReplay)
	return out, isReplay, nil
}

func (s *WithdrawalService) ConfirmWithdrawal(ctx context.Context, withdrawalID int64) (domain.Withdrawal, bool, error) {
	if withdrawalID <= 0 {
		return domain.Withdrawal{}, false, ErrWithdrawalNotFound
	}

	var out domain.Withdrawal
	var isReplay bool
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		withdrawal, err := s.withdrawalRepository.GetByIDForUpdate(ctx, tx, withdrawalID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrWithdrawalNotFound
			}
			return err
		}

		switch withdrawal.Status {
		case WithdrawalConfirmed:
			out = withdrawal
			isReplay = true
			return nil
		case WithdrawalPending:
			updated, updateErr := s.withdrawalRepository.MarkConfirmed(ctx, tx, withdrawalID)
			if updateErr != nil {
				return updateErr
			}

			if err = s.ledgerRepository.Create(ctx, tx, domain.LedgerEntry{
				WithdrawalID: updated.ID,
				UserID:       updated.UserID,
				EntryType:    LedgerConfirmType,
				AmountDelta:  0,
			}); err != nil {
				return err
			}

			out = updated
			return nil
		default:
			return ErrInvalidStatus
		}
	})
	if err != nil {
		s.logger.Error(ctx, "withdrawal_confirm_failed", "withdrawal_id", withdrawalID, "error", err.Error())
		return domain.Withdrawal{}, false, err
	}

	s.logger.Info(ctx, "withdrawal_confirmed", "withdrawal_id", out.ID, "replay", isReplay, "status", out.Status)
	return out, isReplay, nil
}

func (s *WithdrawalService) GetWithdrawalByID(ctx context.Context, id int64) (domain.Withdrawal, error) {
	if id <= 0 {
		return domain.Withdrawal{}, repository.ErrNotFound
	}
	return s.withdrawalRepository.GetByID(ctx, id)
}

func (s *WithdrawalService) logCreateSuccess(ctx context.Context, withdrawal domain.Withdrawal, replay bool) {
	s.logger.Info(
		ctx,
		"withdrawal_created",
		"withdrawal_id", withdrawal.ID,
		"user_id", withdrawal.UserID,
		"amount", withdrawal.Amount,
		"status", withdrawal.Status,
		"replay", replay,
	)
}

func (s *WithdrawalService) logCreateFailure(ctx context.Context, input CreateWithdrawalInput, err error) {
	s.logger.Error(
		ctx,
		"withdrawal_create_failed",
		"user_id", input.UserID,
		"amount", input.Amount,
		"currency", input.Currency,
		"destination", input.Destination,
		"idempotency_key", input.IdempotencyKey,
		"error", err.Error(),
	)
}

func buildPayloadHash(input CreateWithdrawalInput) string {
	payload := fmt.Sprintf("%d|%d|%s|%s", input.UserID, input.Amount, input.Currency, input.Destination)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}
