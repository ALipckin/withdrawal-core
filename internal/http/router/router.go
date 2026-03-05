package router

import (
	"backend/internal/http/handler"
	"backend/internal/http/middleware"
	"net/http"

	"github.com/gorilla/mux"
)

func New(withdrawalHandler *handler.WithdrawalHandler, bearerToken string) http.Handler {
	r := mux.NewRouter()
	api := r.PathPrefix("/v1").Subrouter()
	api.Use(middleware.BearerAuth(bearerToken))

	api.HandleFunc("/withdrawals", withdrawalHandler.CreateWithdrawal).Methods(http.MethodPost)
	api.HandleFunc("/withdrawals/{id}", withdrawalHandler.GetWithdrawal).Methods(http.MethodGet)

	return r
}
