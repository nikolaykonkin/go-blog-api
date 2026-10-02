package handler

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/internal/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newCommentRouter(h *CommentHandler, authUserID *int) *chi.Mux {
	r := chi.NewRouter()

	wrap := func(next http.HandlerFunc) http.HandlerFunc {
		if authUserID == nil {
			return next
		}
		return withUser(*authUserID)(next)
	}

	r.Get("/api/posts/{id}/comments", h.GetByPost)
	r.Post("/api/posts/{postId}/comments", wrap(h.Create))
	r.Get("/api/comments/{id}", h.GetByID)
	r.Put("/api/comments/{id}", wrap(h.Update))
	r.Delete("/api/comments/{id}", wrap(h.Delete))

	return r
}

func newTestCommentHandler(commentRepo *mockCommentRepo, postRepo *mockPostRepo, userRepo *mockUserRepo) *CommentHandler {
	commentService := service.NewCommentService(commentRepo, postRepo, userRepo)
	userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
	return NewCommentHandler(commentService, userService)
}

func TestCommentHandler_Create(t *testing.T) {
	t.Run("success returns 201 and takes the post ID from the URL, not the body", func(t *testing.T) {
		var createdPostID int
		commentRepo := &mockCommentRepo{
			createFunc: func(ctx context.Context, comment *model.Comment) error {
				comment.ID = 77
				createdPostID = comment.PostID
				return nil
			},
		}
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id, Username: "commenter"}, nil
			},
		}
		h := newTestCommentHandler(commentRepo, postRepo, userRepo)
		userID := 5
		router := newCommentRouter(h, &userID)

		// Тело пытается подменить post_id на 999, но параметр маршрута (postId=1)
		// должен победить: см. CommentCreateRequest.PostID с json:"-"
		body := `{"content":"nice post!","post_id":999}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts/1/comments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
		if createdPostID != 1 {
			t.Fatalf("expected the comment's post ID to come from the URL (1), got %d", createdPostID)
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		h := newTestCommentHandler(&mockCommentRepo{}, &mockPostRepo{}, &mockUserRepo{})
		router := newCommentRouter(h, nil)

		body := `{"content":"hi"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts/1/comments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("commenting on a nonexistent post returns 404", func(t *testing.T) {
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return false, nil },
		}
		h := newTestCommentHandler(&mockCommentRepo{}, postRepo, &mockUserRepo{})
		userID := 1
		router := newCommentRouter(h, &userID)

		body := `{"content":"hi"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts/999/comments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("empty content fails validation with 400", func(t *testing.T) {
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
		}
		h := newTestCommentHandler(&mockCommentRepo{}, postRepo, &mockUserRepo{})
		userID := 1
		router := newCommentRouter(h, &userID)

		body := `{"content":"   "}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts/1/comments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid post ID in URL returns 400", func(t *testing.T) {
		h := newTestCommentHandler(&mockCommentRepo{}, &mockPostRepo{}, &mockUserRepo{})
		userID := 1
		router := newCommentRouter(h, &userID)

		body := `{"content":"hi"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts/not-a-number/comments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestCommentHandler_GetByPost(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByPostIDFunc: func(ctx context.Context, postID int, limit, offset int) ([]*model.Comment, error) {
			return []*model.Comment{{ID: 1, PostID: postID}, {ID: 2, PostID: postID}}, nil
		},
		getCountByPostIDFunc: func(ctx context.Context, postID int) (int, error) {
			return 2, nil
		},
	}
	postRepo := &mockPostRepo{
		existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
	}
	userRepo := &mockUserRepo{
		getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}
	h := newTestCommentHandler(commentRepo, postRepo, userRepo)
	router := newCommentRouter(h, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/1/comments?limit=20&offset=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp commentsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 2 || len(resp.Comments) != 2 || resp.PostID != 1 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestCommentHandler_GetByID(t *testing.T) {
	t.Run("found returns 200", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, Content: "found", AuthorID: 1}, nil
			},
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id}, nil
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, userRepo)
		router := newCommentRouter(h, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/comments/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return nil, repository.ErrCommentNotFound
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, &mockUserRepo{})
		router := newCommentRouter(h, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/comments/999", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestCommentHandler_Update(t *testing.T) {
	t.Run("author can update their own comment", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1, Content: "old"}, nil
			},
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id}, nil
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, userRepo)
		userID := 1
		router := newCommentRouter(h, &userID)

		body := `{"content":"updated"}`
		req := httptest.NewRequest(http.MethodPut, "/api/comments/1", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("editing someone else's comment returns 403", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, &mockUserRepo{})
		attackerID := 999
		router := newCommentRouter(h, &attackerID)

		body := `{"content":"hijack"}`
		req := httptest.NewRequest(http.MethodPut, "/api/comments/1", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestCommentHandler_Delete(t *testing.T) {
	t.Run("author can delete their own comment", func(t *testing.T) {
		deleted := false
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
			deleteFunc: func(ctx context.Context, id int) error {
				deleted = true
				return nil
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, &mockUserRepo{})
		userID := 1
		router := newCommentRouter(h, &userID)

		req := httptest.NewRequest(http.MethodDelete, "/api/comments/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
		}
		if !deleted {
			t.Fatal("expected the comment to have been deleted")
		}
	})

	t.Run("deleting someone else's comment returns 403", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
		}
		h := newTestCommentHandler(commentRepo, &mockPostRepo{}, &mockUserRepo{})
		attackerID := 999
		router := newCommentRouter(h, &attackerID)

		req := httptest.NewRequest(http.MethodDelete, "/api/comments/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
