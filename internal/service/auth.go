package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *repository.PostgresDB
	jwtSecret []byte
}

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func NewAuthService(repo *repository.PostgresDB, secret string) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: []byte(secret),
	}
}

func (s *AuthService) Register(ctx context.Context, login, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	id, err := s.repo.CreateUser(ctx, login, string(hash))
	if err != nil {
		return "", err
	}

	return s.generateTokenByID(id)
}

func (s *AuthService) Login(ctx context.Context, login, password string) (string, error) {
	id, hash, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", errors.New("invalid password")
	}

	return s.generateTokenByID(id)
}

func (s *AuthService) generateTokenByID(userID int64) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateToken(tokenString string) (int64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}
	return claims.UserID, nil
}

func GenerateJWTSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
