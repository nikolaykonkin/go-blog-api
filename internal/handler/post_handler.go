package handler

import (
	"blog-api/internal/model"
	"blog-api/internal/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type PostHandler struct {
	postService *service.PostService
	userService *service.UserService
}

// NewPostHandler принимает еще и userService: он заполняет PostResponse.Author
// (см. buildPostResponse), а напрямую в repository handler ходить не должен
func NewPostHandler(postService *service.PostService, userService *service.UserService) *PostHandler {
	return &PostHandler{
		postService: postService,
		userService: userService,
	}
}

// postsListResponse — список постов; limit и offset — значения, которые реально применил сервис
type postsListResponse struct {
	Posts  []model.PostResponse `json:"posts"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// Create обрабатывает POST /api/posts, требует аутентификации
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.PostCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	post, err := h.postService.Create(r.Context(), userID, &req)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, buildPostResponse(r.Context(), h.userService, post))
}

// GetByID обрабатывает GET /api/posts/{id}
func (h *PostHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	post, err := h.postService.GetByID(r.Context(), id)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildPostResponse(r.Context(), h.userService, post))
}

// GetAll обрабатывает GET /api/posts?limit=10&offset=0
func (h *PostHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pagination := parsePagination(r.URL.Query())

	posts, page, err := h.postService.GetAll(r.Context(), pagination.Limit, pagination.Offset)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	resp := postsListResponse{
		Posts:  make([]model.PostResponse, 0, len(posts)),
		Total:  page.Total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}
	for _, p := range posts {
		resp.Posts = append(resp.Posts, buildPostResponse(r.Context(), h.userService, p))
	}

	writeJSON(w, http.StatusOK, resp)
}

// Update обрабатывает PUT /api/posts/{id}, требует аутентификации, только автор
func (h *PostHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	var req model.PostUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	post, err := h.postService.Update(r.Context(), id, userID, &req)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildPostResponse(r.Context(), h.userService, post))
}

// Delete обрабатывает DELETE /api/posts/{id}, требует аутентификации, только автор
func (h *PostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	if err := h.postService.Delete(r.Context(), id, userID); err != nil {
		mapServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetByAuthor обрабатывает GET /api/posts/author/{authorID}?limit=10&offset=0
func (h *PostHandler) GetByAuthor(w http.ResponseWriter, r *http.Request) {
	authorID, err := strconv.Atoi(chi.URLParam(r, "authorID"))
	if err != nil {
		writeError(w, "Invalid author ID", http.StatusBadRequest)
		return
	}

	pagination := parsePagination(r.URL.Query())

	posts, page, err := h.postService.GetByAuthor(r.Context(), authorID, pagination.Limit, pagination.Offset)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	resp := postsListResponse{
		Posts:  make([]model.PostResponse, 0, len(posts)),
		Total:  page.Total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}
	for _, p := range posts {
		resp.Posts = append(resp.Posts, buildPostResponse(r.Context(), h.userService, p))
	}

	writeJSON(w, http.StatusOK, resp)
}
