package main

import (
	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/http/handler"
	"backend/internal/http/router"
	"backend/internal/logger"
	"backend/internal/repository"
	"backend/internal/service"
	"log"
	"net/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer db.Close()

	txManager := repository.NewPostgresTxManager(db)
	userRepo := repository.NewPostgresUserRepository()
	withdrawalRepo := repository.NewPostgresWithdrawalRepository(db)
	ledgerRepo := repository.NewPostgresLedgerRepository()
	eventLogger := logger.New()

	withdrawalService := service.NewWithdrawalService(txManager, userRepo, withdrawalRepo, ledgerRepo, eventLogger)
	withdrawalHandler := handler.NewWithdrawalHandler(withdrawalService)

	server := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router.New(withdrawalHandler, cfg.AuthBearerToken),
	}

	log.Printf("server started on :%s", cfg.AppPort)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run http server: %v", err)
	}
}
