package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

type errorResponse struct {
	Error string `json:"error"`
}

func BearerAuth(expectedToken string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := r.Header.Get("Authorization")
			parts := strings.SplitN(authorization, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != expectedToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
