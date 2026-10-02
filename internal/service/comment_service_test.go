package service

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"context"
	"errors"
	"testing"
)

func TestCommentService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var created *model.Comment
		commentRepo := &mockCommentRepo{
			createFunc: func(ctx context.Context, comment *model.Comment) error {
				comment.ID = 10
				created = comment
				return nil
			},
		}
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) {
				return true, nil
			},
		}
		svc := NewCommentService(commentRepo, postRepo, &mockUserRepo{})

		comment, err := svc.Create(ctx, 9, &model.CommentCreateRequest{Content: "nice!", PostID: 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if comment.ID != 10 {
			t.Fatalf("expected comment ID 10, got %d", comment.ID)
		}
		if created.AuthorID != 9 || created.PostID != 3 {
			t.Fatalf("unexpected created comment: %+v", created)
		}
	})

	t.Run("post does not exist", func(t *testing.T) {
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) {
				return false, nil
			},
		}
		svc := NewCommentService(&mockCommentRepo{}, postRepo, &mockUserRepo{})

		_, err := svc.Create(ctx, 1, &model.CommentCreateRequest{Content: "hi", PostID: 999})
		if !errors.Is(err, ErrPostNotExists) {
			t.Fatalf("expected ErrPostNotExists, got %v", err)
		}
	})

	validationCases := []struct {
		name string
		req  model.CommentCreateRequest
	}{
		{"empty content", model.CommentCreateRequest{Content: "   ", PostID: 1}},
		{"content too long", model.CommentCreateRequest{Content: string(make([]byte, 1001)), PostID: 1}},
		{"missing post id", model.CommentCreateRequest{Content: "hello", PostID: 0}},
	}
	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			postRepo := &mockPostRepo{
				existsFunc: func(ctx context.Context, id int) (bool, error) {
					t.Fatal("post existence should not be checked once validation has already failed")
					return false, nil
				},
			}
			svc := NewCommentService(&mockCommentRepo{}, postRepo, &mockUserRepo{})

			req := tc.req
			_, err := svc.Create(ctx, 1, &req)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestCommentService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, Content: "found"}, nil
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		comment, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if comment.Content != "found" {
			t.Fatalf("unexpected comment: %+v", comment)
		}
	})

	t.Run("not found", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return nil, repository.ErrCommentNotFound
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		_, err := svc.GetByID(ctx, 999)
		if !errors.Is(err, ErrCommentNotFound) {
			t.Fatalf("expected ErrCommentNotFound, got %v", err)
		}
	})
}

func TestCommentService_GetByPost(t *testing.T) {
	ctx := context.Background()

	t.Run("defaults limit to 20 and caps offset at 0", func(t *testing.T) {
		commentRepo := &mockCommentRepo{}
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
		}
		svc := NewCommentService(commentRepo, postRepo, &mockUserRepo{})

		_, _, err := svc.GetByPost(ctx, 1, 0, -5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if commentRepo.capturedLimit != 20 {
			t.Errorf("expected default limit 20, got %d", commentRepo.capturedLimit)
		}
		if commentRepo.capturedOffset != 0 {
			t.Errorf("expected offset clamped to 0, got %d", commentRepo.capturedOffset)
		}
	})

	t.Run("caps limit at the global max", func(t *testing.T) {
		commentRepo := &mockCommentRepo{}
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
		}
		svc := NewCommentService(commentRepo, postRepo, &mockUserRepo{})

		_, _, err := svc.GetByPost(ctx, 1, 1000, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if commentRepo.capturedLimit != maxLimit {
			t.Errorf("expected limit capped at %d, got %d", maxLimit, commentRepo.capturedLimit)
		}
	})

	t.Run("post does not exist", func(t *testing.T) {
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return false, nil },
		}
		svc := NewCommentService(&mockCommentRepo{}, postRepo, &mockUserRepo{})

		_, _, err := svc.GetByPost(ctx, 999, 10, 0)
		if !errors.Is(err, ErrPostNotExists) {
			t.Fatalf("expected ErrPostNotExists, got %v", err)
		}
	})

	t.Run("returns comments and total count", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByPostIDFunc: func(ctx context.Context, postID int, limit, offset int) ([]*model.Comment, error) {
				return []*model.Comment{{ID: 1}, {ID: 2}}, nil
			},
			getCountByPostIDFunc: func(ctx context.Context, postID int) (int, error) {
				return 2, nil
			},
		}
		postRepo := &mockPostRepo{
			existsFunc: func(ctx context.Context, id int) (bool, error) { return true, nil },
		}
		svc := NewCommentService(commentRepo, postRepo, &mockUserRepo{})

		// total теперь внутри PaginationResult вместе с примененными limit и offset
		comments, page, err := svc.GetByPost(ctx, 1, 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(comments) != 2 || page.Total != 2 {
			t.Fatalf("expected 2 comments and total 2, got %d comments, total %d", len(comments), page.Total)
		}
	})
}

func TestCommentService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("success when caller is the author", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1, Content: "old"}, nil
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		comment, err := svc.Update(ctx, 1, 1, &model.CommentUpdateRequest{Content: "new"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if comment.Content != "new" {
			t.Fatalf("expected content to be updated, got %q", comment.Content)
		}
	})

	t.Run("forbidden when caller is not the author", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 999, &model.CommentUpdateRequest{Content: "new"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return nil, repository.ErrCommentNotFound
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 1, &model.CommentUpdateRequest{Content: "new"})
		if !errors.Is(err, ErrCommentNotFound) {
			t.Fatalf("expected ErrCommentNotFound, got %v", err)
		}
	})

	t.Run("validation error on empty content", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		_, err := svc.Update(ctx, 1, 1, &model.CommentUpdateRequest{Content: "   "})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
}

func TestCommentService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success when caller is the author", func(t *testing.T) {
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
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		if err := svc.Delete(ctx, 1, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleted {
			t.Fatal("expected repo.Delete to be called")
		}
	})

	t.Run("forbidden when caller is not the author", func(t *testing.T) {
		commentRepo := &mockCommentRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.Comment, error) {
				return &model.Comment{ID: id, AuthorID: 1}, nil
			},
			deleteFunc: func(ctx context.Context, id int) error {
				t.Fatal("repo.Delete should not be called when the caller is forbidden")
				return nil
			},
		}
		svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

		err := svc.Delete(ctx, 1, 999)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestCommentService_GetByAuthor(t *testing.T) {
	ctx := context.Background()

	commentRepo := &mockCommentRepo{
		getByAuthorIDFunc: func(ctx context.Context, authorID int, limit, offset int) ([]*model.Comment, error) {
			return []*model.Comment{{ID: 1, AuthorID: authorID}}, nil
		},
		getTotalCountByAuthorIDFunc: func(ctx context.Context, authorID int) (int, error) {
			return 1, nil
		},
	}
	svc := NewCommentService(commentRepo, &mockPostRepo{}, &mockUserRepo{})

	// total теперь внутри PaginationResult вместе с примененными limit и offset
	comments, page, err := svc.GetByAuthor(ctx, 3, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 || page.Total != 1 {
		t.Fatalf("expected 1 comment and total 1, got %d comments, total %d", len(comments), page.Total)
	}
	if commentRepo.capturedLimit != defaultLimit {
		t.Fatalf("expected default limit %d (normalizePagination shared with posts), got %d", defaultLimit, commentRepo.capturedLimit)
	}
}
