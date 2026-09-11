package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/DenisChesnokov/gophermart/internal/service"
)

type Handler struct {
	auth *service.AuthService
}

func New(auth *service.AuthService) *Handler {
	return &Handler{auth: auth}
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.auth.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			http.Error(w, "login already exists", http.StatusConflict)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.auth.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
