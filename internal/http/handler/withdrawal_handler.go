package handler

import (
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/service"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type WithdrawalHandler struct {
	service *service.WithdrawalService
}

type createWithdrawalRequest struct {
	UserID         int64  `json:"user_id"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	Destination    string `json:"destination"`
	IdempotencyKey string `json:"idempotency_key"`
}

type withdrawalResponse struct {
	ID             int64   `json:"id"`
	UserID         int64   `json:"user_id"`
	Amount         int64   `json:"amount"`
	Currency       string  `json:"currency"`
	Destination    string  `json:"destination"`
	IdempotencyKey string  `json:"idempotency_key"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	ConfirmedAt    *string `json:"confirmed_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewWithdrawalHandler(service *service.WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{service: service}
}

func (h *WithdrawalHandler) CreateWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req createWithdrawalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	created, _, err := h.service.CreateWithdrawal(r.Context(), service.CreateWithdrawalInput{
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Destination:    req.Destination,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapWithdrawal(created))
}

func (h *WithdrawalHandler) GetWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	withdrawal, err := h.service.GetWithdrawalByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "withdrawal not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, mapWithdrawal(withdrawal))
}

func (h *WithdrawalHandler) ConfirmWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	withdrawal, _, err := h.service.ConfirmWithdrawal(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapWithdrawal(withdrawal))
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idRaw := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid withdrawal id")
		return 0, false
	}
	return id, true
}

func (h *WithdrawalHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidAmount),
		errors.Is(err, service.ErrInvalidCurrency),
		errors.Is(err, service.ErrInvalidIdempotencyKey),
		errors.Is(err, service.ErrInvalidDestination):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrIdempotencyConflict):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, service.ErrUserNotFound),
		errors.Is(err, service.ErrWithdrawalNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidStatus):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func mapWithdrawal(withdrawal domain.Withdrawal) withdrawalResponse {
	var confirmedAt *string
	if withdrawal.ConfirmedAt != nil {
		timestamp := withdrawal.ConfirmedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		confirmedAt = &timestamp
	}

	return withdrawalResponse{
		ID:             withdrawal.ID,
		UserID:         withdrawal.UserID,
		Amount:         withdrawal.Amount,
		Currency:       withdrawal.Currency,
		Destination:    withdrawal.Destination,
		IdempotencyKey: withdrawal.IdempotencyKey,
		Status:         withdrawal.Status,
		CreatedAt:      withdrawal.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ConfirmedAt:    confirmedAt,
	}
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
