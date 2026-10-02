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

// newPostRouter повторяет маршруты постов из cmd/api/main.go
// authUserID имитирует авторизованного пользователя, nil — анонимного
func newPostRouter(h *PostHandler, authUserID *int) *chi.Mux {
	r := chi.NewRouter()

	wrap := func(next http.HandlerFunc) http.HandlerFunc {
		if authUserID == nil {
			return next
		}
		return withUser(*authUserID)(next)
	}

	r.Get("/api/posts", h.GetAll)
	r.Get("/api/posts/{id}", h.GetByID)
	r.Get("/api/posts/author/{authorID}", h.GetByAuthor)
	r.Post("/api/posts", wrap(h.Create))
	r.Put("/api/posts/{id}", wrap(h.Update))
	r.Delete("/api/posts/{id}", wrap(h.Delete))

	return r
}

func newTestPostHandler(postRepo *mockPostRepo, userRepo *mockUserRepo) *PostHandler {
	postService := service.NewPostService(postRepo, userRepo)
	userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
	return NewPostHandler(postService, userService)
}

func TestPostHandler_Create(t *testing.T) {
	t.Run("success returns 201 with the created post and author", func(t *testing.T) {
		postRepo := &mockPostRepo{
			createFunc: func(ctx context.Context, post *model.Post) error {
				post.ID = 55
				return nil
			},
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id, Username: "author1"}, nil
			},
		}
		h := newTestPostHandler(postRepo, userRepo)
		userID := 7
		router := newPostRouter(h, &userID)

		body := `{"title":"My Post","content":"Post body"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp model.PostResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ID != 55 || resp.Title != "My Post" {
			t.Fatalf("unexpected post response: %+v", resp)
		}
		if resp.Author.Username != "author1" {
			t.Fatalf("expected the response to be enriched with author info, got: %+v", resp.Author)
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		h := newTestPostHandler(&mockPostRepo{}, &mockUserRepo{})
		router := newPostRouter(h, nil)

		body := `{"title":"My Post","content":"Post body"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		h := newTestPostHandler(&mockPostRepo{}, &mockUserRepo{})
		userID := 1
		router := newPostRouter(h, &userID)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader("{bad json"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty title fails validation with 400", func(t *testing.T) {
		h := newTestPostHandler(&mockPostRepo{}, &mockUserRepo{})
		userID := 1
		router := newPostRouter(h, &userID)

		body := `{"title":"","content":"content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestPostHandler_GetByID(t *testing.T) {
	t.Run("found returns 200", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, Title: "Found Post", AuthorID: 1}, nil
			},
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id, Username: "author1"}, nil
			},
		}
		h := newTestPostHandler(postRepo, userRepo)
		router := newPostRouter(h, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/42", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return nil, repository.ErrPostNotFound
			},
		}
		h := newTestPostHandler(postRepo, &mockUserRepo{})
		router := newPostRouter(h, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/999", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-numeric id returns 400", func(t *testing.T) {
		h := newTestPostHandler(&mockPostRepo{}, &mockUserRepo{})
		router := newPostRouter(h, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/not-a-number", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestPostHandler_GetAll(t *testing.T) {
	postRepo := &mockPostRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) ([]*model.Post, error) {
			return []*model.Post{{ID: 1, AuthorID: 1}, {ID: 2, AuthorID: 1}}, nil
		},
		getTotalCountFunc: func(ctx context.Context) (int, error) {
			return 2, nil
		},
	}
	userRepo := &mockUserRepo{
		getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}
	h := newTestPostHandler(postRepo, userRepo)
	router := newPostRouter(h, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/posts?limit=5&offset=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp postsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 2 || len(resp.Posts) != 2 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestPostHandler_Update(t *testing.T) {
	t.Run("author can update their own post", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1, Title: "old", Content: "old"}, nil
			},
		}
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id}, nil
			},
		}
		h := newTestPostHandler(postRepo, userRepo)
		userID := 1
		router := newPostRouter(h, &userID)

		body := `{"title":"new title","content":"new content"}`
		req := httptest.NewRequest(http.MethodPut, "/api/posts/1", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("editing someone else's post returns 403", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1, Title: "old", Content: "old"}, nil
			},
		}
		h := newTestPostHandler(postRepo, &mockUserRepo{})
		attackerID := 999
		router := newPostRouter(h, &attackerID)

		body := `{"title":"hijack attempt","content":"..."}`
		req := httptest.NewRequest(http.MethodPut, "/api/posts/1", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("updating a nonexistent post returns 404", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return nil, repository.ErrPostNotFound
			},
		}
		h := newTestPostHandler(postRepo, &mockUserRepo{})
		userID := 1
		router := newPostRouter(h, &userID)

		body := `{"title":"new","content":"new"}`
		req := httptest.NewRequest(http.MethodPut, "/api/posts/404", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestPostHandler_Delete(t *testing.T) {
	t.Run("author can delete their own post", func(t *testing.T) {
		deleted := false
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
			deleteFunc: func(ctx context.Context, id int) error {
				deleted = true
				return nil
			},
		}
		h := newTestPostHandler(postRepo, &mockUserRepo{})
		userID := 1
		router := newPostRouter(h, &userID)

		req := httptest.NewRequest(http.MethodDelete, "/api/posts/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
		}
		if !deleted {
			t.Fatal("expected the post to have been deleted")
		}
	})

	t.Run("deleting someone else's post returns 403", func(t *testing.T) {
		postRepo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
		}
		h := newTestPostHandler(postRepo, &mockUserRepo{})
		attackerID := 999
		router := newPostRouter(h, &attackerID)

		req := httptest.NewRequest(http.MethodDelete, "/api/posts/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestPostHandler_GetByAuthor(t *testing.T) {
	postRepo := &mockPostRepo{
		getByAuthorIDFunc: func(ctx context.Context, authorID int, limit, offset int) ([]*model.Post, error) {
			return []*model.Post{{ID: 1, AuthorID: authorID}}, nil
		},
		getTotalCountByAuthorIDFunc: func(ctx context.Context, authorID int) (int, error) {
			return 1, nil
		},
	}
	userRepo := &mockUserRepo{
		getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}
	h := newTestPostHandler(postRepo, userRepo)
	router := newPostRouter(h, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/author/3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
