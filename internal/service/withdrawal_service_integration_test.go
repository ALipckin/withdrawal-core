package service_test

import (
	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/repository"
	"backend/internal/service"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestCreateWithdrawalSuccess(t *testing.T) {
	db, svc, _ := testDependencies(t)
	userID := seedUser(t, db, 1000)

	withdrawal, replay, err := svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
		UserID:         userID,
		Amount:         400,
		Currency:       service.CurrencyUSDT,
		Destination:    "wallet-1",
		IdempotencyKey: "create-success",
	})
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}
	if replay {
		t.Fatalf("expected non-replay response")
	}
	if withdrawal.ID == 0 {
		t.Fatalf("expected withdrawal id to be generated")
	}
	if withdrawal.Status != service.WithdrawalPending {
		t.Fatalf("unexpected status: %s", withdrawal.Status)
	}

	balance := getBalance(t, db, userID)
	if balance != 600 {
		t.Fatalf("expected balance 600, got %d", balance)
	}
}

func TestCreateWithdrawalInsufficientBalance(t *testing.T) {
	db, svc, _ := testDependencies(t)
	userID := seedUser(t, db, 100)

	_, _, err := svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
		UserID:         userID,
		Amount:         101,
		Currency:       service.CurrencyUSDT,
		Destination:    "wallet-2",
		IdempotencyKey: "insufficient-balance",
	})
	if !errors.Is(err, service.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestCreateWithdrawalIdempotency(t *testing.T) {
	db, svc, withdrawalRepo := testDependencies(t)
	userID := seedUser(t, db, 1000)

	first, replay, err := svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
		UserID:         userID,
		Amount:         250,
		Currency:       service.CurrencyUSDT,
		Destination:    "wallet-3",
		IdempotencyKey: "idem-key",
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if replay {
		t.Fatalf("first call must not be replay")
	}

	second, replay, err := svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
		UserID:         userID,
		Amount:         250,
		Currency:       service.CurrencyUSDT,
		Destination:    "wallet-3",
		IdempotencyKey: "idem-key",
	})
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if !replay {
		t.Fatalf("second call should be replay")
	}
	if first.ID != second.ID {
		t.Fatalf("expected same withdrawal id for replay, got %d and %d", first.ID, second.ID)
	}

	_, _, err = svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
		UserID:         userID,
		Amount:         251,
		Currency:       service.CurrencyUSDT,
		Destination:    "wallet-3",
		IdempotencyKey: "idem-key",
	})
	if !errors.Is(err, service.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}

	count, err := withdrawalRepo.CountByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("count withdrawals: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one withdrawal, got %d", count)
	}

	balance := getBalance(t, db, userID)
	if balance != 750 {
		t.Fatalf("expected balance 750, got %d", balance)
	}
}

func TestCreateWithdrawalConcurrency(t *testing.T) {
	db, svc, withdrawalRepo := testDependencies(t)
	userID := seedUser(t, db, 100)

	type result struct {
		err error
	}
	results := make(chan result, 2)

	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _, err := svc.CreateWithdrawal(context.Background(), service.CreateWithdrawalInput{
				UserID:         userID,
				Amount:         80,
				Currency:       service.CurrencyUSDT,
				Destination:    "wallet-concurrent",
				IdempotencyKey: fmt.Sprintf("concurrent-%d", index),
			})
			results <- result{err: err}
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	insufficientCount := 0
	for item := range results {
		switch {
		case item.err == nil:
			successCount++
		case errors.Is(item.err, service.ErrInsufficientBalance):
			insufficientCount++
		default:
			t.Fatalf("unexpected error: %v", item.err)
		}
	}

	if successCount != 1 || insufficientCount != 1 {
		t.Fatalf("expected 1 success and 1 insufficient error, got success=%d insufficient=%d", successCount, insufficientCount)
	}

	count, err := withdrawalRepo.CountByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("count withdrawals: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one withdrawal after race, got %d", count)
	}

	balance := getBalance(t, db, userID)
	if balance != 20 {
		t.Fatalf("expected balance 20, got %d", balance)
	}
}

func testDependencies(t *testing.T) (*sql.DB, *service.WithdrawalService, *repository.PostgresWithdrawalRepository) {
	t.Helper()

	cfg := &config.Config{
		DBUser:          envOrDefault("DB_USER", "user_test"),
		DBPassword:      envOrDefault("DB_PASSWORD", "user_password"),
		DBHost:          envOrDefault("DB_HOST", "db"),
		DBPort:          envOrDefault("DB_PORT", "5432"),
		DBName:          envOrDefault("DB_DATABASE", "withdrawal_core"),
		MigrationsPath:  envOrDefault("MIGRATIONS_PATH", "file:///workspace/internal/database/migrations"),
		AuthBearerToken: envOrDefault("AUTH_BEARER_TOKEN", "test-token"),
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err = db.Exec(`TRUNCATE TABLE withdrawals, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	txManager := repository.NewPostgresTxManager(db)
	userRepo := repository.NewPostgresUserRepository()
	withdrawalRepo := repository.NewPostgresWithdrawalRepository(db)

	svc := service.NewWithdrawalService(txManager, userRepo, withdrawalRepo)
	return db, svc, withdrawalRepo
}

func seedUser(t *testing.T, db *sql.DB, balance int64) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRow(`INSERT INTO users (balance) VALUES ($1) RETURNING id`, balance).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func getBalance(t *testing.T, db *sql.DB, userID int64) int64 {
	t.Helper()

	var balance int64
	if err := db.QueryRow(`SELECT balance FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
		t.Fatalf("get balance: %v", err)
	}
	return balance
}

func envOrDefault(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
