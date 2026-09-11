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

	return middleware.Auth(authService)(mux)
}
