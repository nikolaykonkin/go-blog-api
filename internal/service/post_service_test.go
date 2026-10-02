package service

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"context"
	"errors"
	"testing"
)

func TestPostService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var created *model.Post
		repo := &mockPostRepo{
			createFunc: func(ctx context.Context, post *model.Post) error {
				post.ID = 42
				created = post
				return nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		post, err := svc.Create(ctx, 7, &model.PostCreateRequest{Title: "Hello", Content: "World"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if post.ID != 42 {
			t.Fatalf("expected post ID 42, got %d", post.ID)
		}
		if created.AuthorID != 7 {
			t.Fatalf("expected AuthorID 7, got %d", created.AuthorID)
		}
	})

	cases := []struct {
		name string
		req  model.PostCreateRequest
	}{
		{"empty title", model.PostCreateRequest{Title: "   ", Content: "content"}},
		{"title too long", model.PostCreateRequest{Title: string(make([]byte, 201)), Content: "content"}},
		{"empty content", model.PostCreateRequest{Title: "title", Content: "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewPostService(&mockPostRepo{}, &mockUserRepo{})
			req := tc.req
			_, err := svc.Create(ctx, 1, &req)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestPostService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, Title: "found"}, nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		post, err := svc.GetByID(ctx, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if post.Title != "found" {
			t.Fatalf("unexpected post: %+v", post)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return nil, repository.ErrPostNotFound
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		_, err := svc.GetByID(ctx, 999)
		if !errors.Is(err, ErrPostNotFound) {
			t.Fatalf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostService_GetAll_PaginationNormalization(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		limit, offset  int
		expectedLimit  int
		expectedOffset int
	}{
		{"zero limit defaults to 10", 0, 0, defaultLimit, 0},
		{"negative limit defaults to 10", -5, 0, defaultLimit, 0},
		{"limit above max is capped at 100", 500, 0, maxLimit, 0},
		{"negative offset is clamped to 0", 10, -20, 10, 0},
		{"within-range values pass through unchanged", 25, 15, 25, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockPostRepo{}
			svc := NewPostService(repo, &mockUserRepo{})

			_, _, err := svc.GetAll(ctx, tt.limit, tt.offset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.capturedLimit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, repo.capturedLimit)
			}
			if repo.capturedOffset != tt.expectedOffset {
				t.Errorf("expected offset %d, got %d", tt.expectedOffset, repo.capturedOffset)
			}
		})
	}
}

func TestPostService_GetAll_ReturnsPostsAndTotal(t *testing.T) {
	ctx := context.Background()
	repo := &mockPostRepo{
		getAllFunc: func(ctx context.Context, limit, offset int) ([]*model.Post, error) {
			return []*model.Post{{ID: 1}, {ID: 2}}, nil
		},
		getTotalCountFunc: func(ctx context.Context) (int, error) {
			return 2, nil
		},
	}
	svc := NewPostService(repo, &mockUserRepo{})

	// total теперь внутри PaginationResult вместе с примененными limit и offset
	posts, page, err := svc.GetAll(ctx, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 || page.Total != 2 {
		t.Fatalf("expected 2 posts and total 2, got %d posts, total %d", len(posts), page.Total)
	}
}

func TestPostService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("success when caller is the author", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1, Title: "old", Content: "old content"}, nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		post, err := svc.Update(ctx, 1, 1, &model.PostUpdateRequest{Title: "new", Content: "new content"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if post.Title != "new" || post.Content != "new content" {
			t.Fatalf("expected post to be updated, got %+v", post)
		}
	})

	t.Run("forbidden when caller is not the author", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 999, &model.PostUpdateRequest{Title: "new", Content: "new content"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return nil, repository.ErrPostNotFound
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 1, &model.PostUpdateRequest{Title: "new", Content: "new content"})
		if !errors.Is(err, ErrPostNotFound) {
			t.Fatalf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("validation error on empty title", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 1, &model.PostUpdateRequest{Title: "", Content: "content"})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
}

func TestPostService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success when caller is the author", func(t *testing.T) {
		deleted := false
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
			deleteFunc: func(ctx context.Context, id int) error {
				deleted = true
				return nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		if err := svc.Delete(ctx, 1, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleted {
			t.Fatal("expected repo.Delete to be called")
		}
	})

	t.Run("forbidden when caller is not the author", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return &model.Post{ID: id, AuthorID: 1}, nil
			},
			deleteFunc: func(ctx context.Context, id int) error {
				t.Fatal("repo.Delete should not be called when the caller is forbidden")
				return nil
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		err := svc.Delete(ctx, 1, 999)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockPostRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Post, error) {
				return nil, repository.ErrPostNotFound
			},
		}
		svc := NewPostService(repo, &mockUserRepo{})

		err := svc.Delete(ctx, 1, 1)
		if !errors.Is(err, ErrPostNotFound) {
			t.Fatalf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostService_GetByAuthor(t *testing.T) {
	ctx := context.Background()

	repo := &mockPostRepo{
		getByAuthorIDFunc: func(ctx context.Context, authorID int, limit, offset int) ([]*model.Post, error) {
			return []*model.Post{{ID: 1, AuthorID: authorID}}, nil
		},
		getTotalCountByAuthorIDFunc: func(ctx context.Context, authorID int) (int, error) {
			return 1, nil
		},
	}
	svc := NewPostService(repo, &mockUserRepo{})

	// total теперь внутри PaginationResult вместе с примененными limit и offset
	posts, page, err := svc.GetByAuthor(ctx, 3, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 1 || page.Total != 1 {
		t.Fatalf("expected 1 post and total 1, got %d posts, total %d", len(posts), page.Total)
	}
	if repo.capturedLimit != defaultLimit {
		t.Fatalf("expected default limit %d to be applied, got %d", defaultLimit, repo.capturedLimit)
	}
}
