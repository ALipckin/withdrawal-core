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
	CurrencyUSDT      = "USDT"
	WithdrawalPending = "pending"
)

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
}

func NewWithdrawalService(
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	withdrawalRepository repository.WithdrawalRepository,
) *WithdrawalService {
	return &WithdrawalService{
		txManager:            txManager,
		userRepository:       userRepository,
		withdrawalRepository: withdrawalRepository,
	}
}

func (s *WithdrawalService) CreateWithdrawal(ctx context.Context, input CreateWithdrawalInput) (domain.Withdrawal, bool, error) {
	if input.Amount <= 0 {
		return domain.Withdrawal{}, false, ErrInvalidAmount
	}
	if input.Currency != CurrencyUSDT {
		return domain.Withdrawal{}, false, ErrInvalidCurrency
	}
	if input.IdempotencyKey == "" {
		return domain.Withdrawal{}, false, ErrInvalidIdempotencyKey
	}
	if input.Destination == "" {
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

		out = created
		return nil
	})
	if err != nil {
		return domain.Withdrawal{}, false, err
	}

	return out, isReplay, nil
}

func (s *WithdrawalService) GetWithdrawalByID(ctx context.Context, id int64) (domain.Withdrawal, error) {
	if id <= 0 {
		return domain.Withdrawal{}, repository.ErrNotFound
	}
	return s.withdrawalRepository.GetByID(ctx, id)
}

func buildPayloadHash(input CreateWithdrawalInput) string {
	payload := fmt.Sprintf("%d|%d|%s|%s", input.UserID, input.Amount, input.Currency, input.Destination)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}
