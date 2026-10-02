// Package handler содержит HTTP-обработчики — верхний слой архитектуры
// Handler -> Service -> Repository
package handler

import (
	"blog-api/internal/middleware"
	"blog-api/internal/model"
	"blog-api/internal/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// errorResponse — единый формат JSON-ошибки для всего API
type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, message string, statusCode int) {
	writeJSON(w, statusCode, errorResponse{Error: message})
}

// getUserIDFromContext делегирует в middleware: ключ контекста определен там же,
// где RequireAuth его ставит, и заводить второй набор ключей в handler незачем
func getUserIDFromContext(ctx context.Context) (int, bool) {
	return middleware.GetUserIDFromContext(ctx)
}

// paginationParams — параметры пагинации из query string без нормализации
// Дефолты и верхние границы применяет сервис, чтобы правило жило в одном месте
type paginationParams struct {
	Limit  int
	Offset int
}

// parsePagination превращает некорректные и отсутствующие значения в 0, а не в ошибку 400:
// сервис сам подставит значение по умолчанию (см. normalizePagination)
func parsePagination(query url.Values) paginationParams {
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))
	return paginationParams{Limit: limit, Offset: offset}
}

// mapServiceError переводит sentinel-ошибки сервисов в HTTP-статус и текст
// Одна функция на все хендлеры, чтобы коды ошибок совпадали по всему API
func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		writeError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrPostNotFound):
		writeError(w, "Post not found", http.StatusNotFound)
	case errors.Is(err, service.ErrCommentNotFound):
		writeError(w, "Comment not found", http.StatusNotFound)
	case errors.Is(err, service.ErrUserNotFound):
		writeError(w, "User not found", http.StatusNotFound)
	case errors.Is(err, service.ErrPostNotExists):
		writeError(w, "Post not found", http.StatusNotFound)
	case errors.Is(err, service.ErrForbidden):
		writeError(w, "You don't have permission to modify this resource", http.StatusForbidden)
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, "Unauthorized", http.StatusUnauthorized)
	case errors.Is(err, service.ErrUserAlreadyExists):
		writeError(w, "User with this email or username already exists", http.StatusConflict)
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, "Invalid email or password", http.StatusUnauthorized)
	default:
		writeError(w, "Internal server error", http.StatusInternalServerError)
	}
}

// buildPostResponse дополняет пост данными автора без email: ответ публичный
// Если автора получить не удалось, пост все равно отдается с пустым Author —
// единичная нестыковка данных не должна ломать весь ответ
func buildPostResponse(ctx context.Context, userService *service.UserService, post *model.Post) model.PostResponse {
	resp := model.PostResponse{
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}

	if author, err := userService.GetByID(ctx, post.AuthorID); err == nil {
		resp.Author = author.ToAuthorResponse()
	}

	return resp
}

// buildCommentResponse — аналог buildPostResponse для комментариев
func buildCommentResponse(ctx context.Context, userService *service.UserService, comment *model.Comment) model.CommentResponse {
	resp := model.CommentResponse{
		ID:        comment.ID,
		Content:   comment.Content,
		PostID:    comment.PostID,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}

	if author, err := userService.GetByID(ctx, comment.AuthorID); err == nil {
		resp.Author = author.ToAuthorResponse()
	}

	return resp
}
