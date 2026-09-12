package handler

import (
	"net/http"

	"github.com/DenisChesnokov/gophermart/internal/middleware"
	"github.com/DenisChesnokov/gophermart/internal/service"
)

func NewRouter(h *Handler, authService *service.AuthService) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", h.Ping)
	mux.HandleFunc("/api/user/register", h.Register)
	mux.HandleFunc("/api/user/login", h.Login)
	mux.HandleFunc("/api/user/orders", h.Orders)
	mux.HandleFunc("/api/user/balance", h.Balance)
	mux.HandleFunc("/api/user/balance/withdraw", h.BalanceWithdraw)
	mux.HandleFunc("/api/user/withdrawals", h.WithdrawalsHandler)

	return middleware.Auth(authService)(mux)
}
