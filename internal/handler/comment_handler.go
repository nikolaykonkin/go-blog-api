package handler

import (
	"blog-api/internal/model"
	"blog-api/internal/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type CommentHandler struct {
	commentService *service.CommentService
	userService    *service.UserService
}

// NewCommentHandler принимает userService по той же причине, что и NewPostHandler:
// он заполняет CommentResponse.Author
func NewCommentHandler(commentService *service.CommentService, userService *service.UserService) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		userService:    userService,
	}
}

// commentsListResponse — список комментариев; limit и offset — значения, которые реально применил сервис
type commentsListResponse struct {
	Comments []model.CommentResponse `json:"comments"`
	Total    int                     `json:"total"`
	Limit    int                     `json:"limit"`
	Offset   int                     `json:"offset"`
	PostID   int                     `json:"post_id"`
}

// Create обрабатывает POST /api/posts/{postId}/comments, требует аутентификации
// ID поста берется из URL, а не из тела: PostID в запросе помечен json:"-",
// поэтому клиент не может его подменить
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	postID, err := strconv.Atoi(chi.URLParam(r, "postId"))
	if err != nil {
		writeError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	var req model.CommentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.PostID = postID

	comment, err := h.commentService.Create(r.Context(), userID, &req)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, buildCommentResponse(r.Context(), h.userService, comment))
}

// GetByID обрабатывает GET /api/comments/{id}
func (h *CommentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	comment, err := h.commentService.GetByID(r.Context(), id)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(r.Context(), h.userService, comment))
}

// GetByPost обрабатывает GET /api/posts/{id}/comments?limit=20&offset=0
func (h *CommentHandler) GetByPost(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	pagination := parsePagination(r.URL.Query())

	comments, page, err := h.commentService.GetByPost(r.Context(), postID, pagination.Limit, pagination.Offset)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	resp := commentsListResponse{
		Comments: make([]model.CommentResponse, 0, len(comments)),
		Total:    page.Total,
		Limit:    page.Limit,
		Offset:   page.Offset,
		PostID:   postID,
	}
	for _, c := range comments {
		resp.Comments = append(resp.Comments, buildCommentResponse(r.Context(), h.userService, c))
	}

	writeJSON(w, http.StatusOK, resp)
}

// Update обрабатывает PUT /api/comments/{id}, требует аутентификации, только автор
func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	var req model.CommentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	comment, err := h.commentService.Update(r.Context(), id, userID, &req)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(r.Context(), h.userService, comment))
}

// Delete обрабатывает DELETE /api/comments/{id}, требует аутентификации, только автор
func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	if err := h.commentService.Delete(r.Context(), id, userID); err != nil {
		mapServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
