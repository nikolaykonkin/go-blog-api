package middleware

import (
	"blog-api/pkg/auth"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// contextKey — собственный тип ключа, чтобы не пересечься с ключами других пакетов
type contextKey string

const (
	UserIDKey    contextKey = "userID"
	UserEmailKey contextKey = "userEmail"
	UserNameKey  contextKey = "username"
	requestIDKey contextKey = "requestID"
)

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
}

func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
	}
}

// RequireAuth кладет user ID, email и username из токена в контекст запроса
func (m *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeJSONError(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			if errors.Is(err, auth.ErrExpiredToken) {
				writeJSONError(w, "Token expired", http.StatusUnauthorized)
				return
			}
			writeJSONError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserNameKey, claims.Username)

		next(w, r.WithContext(ctx))
	}
}

// OptionalAuth кладет данные из токена в контекст, если он валиден, и не требует его наличия
func (m *AuthMiddleware) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			next(w, r)
			return
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Невалидный или истекший токен не повод для отказа: идем дальше как анонимный
			next(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserNameKey, claims.Username)

		next(w, r.WithContext(ctx))
	}
}

// extractToken достает токен из заголовка Authorization вида "Bearer <token>"
func extractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimPrefix(header, prefix)
}

func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDKey).(int)
	return userID, ok
}

func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

func GetUsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(UserNameKey).(string)
	return username, ok
}

// errorResponse совпадает с форматом ошибок в handler и дублируется здесь,
// потому что handler импортирует middleware, а не наоборот
type errorResponse struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse{Error: message})
}

// Chain применяет middleware в порядке аргументов: в Chain(h, A, B)
// сначала отработает A, затем B, затем сам h
func Chain(handler http.HandlerFunc, middlewares ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
