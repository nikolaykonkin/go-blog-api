package service

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrPostNotFound = errors.New("post not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Значения пагинации по умолчанию и верхняя граница — общие для сервисов пакета
const (
	defaultLimit = 10
	maxLimit     = 100
)

// normalizePagination заменяет limit <= 0 на defaultLimit, режет его до maxLimit
// и не допускает отрицательный offset
func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// PaginationResult — limit и offset, которые сервис реально применил после нормализации,
// и общее число записей; хендлеры отдают их клиенту вместо сырых значений из query string
type PaginationResult struct {
	Limit  int
	Offset int
	Total  int
}

type PostService struct {
	postRepo repository.PostRepository
	userRepo repository.UserRepository
}

func NewPostService(postRepo repository.PostRepository, userRepo repository.UserRepository) *PostService {
	return &PostService{
		postRepo: postRepo,
		userRepo: userRepo,
	}
}

func (s *PostService) Create(ctx context.Context, userID int, req *model.PostCreateRequest) (*model.Post, error) {
	if err := validatePostCreateRequest(req); err != nil {
		return nil, err
	}

	// Сохраняем значения без пробелов по краям: валидация проверяла именно их
	post := &model.Post{
		Title:    strings.TrimSpace(req.Title),
		Content:  strings.TrimSpace(req.Content),
		AuthorID: userID,
	}

	if err := s.postRepo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return post, nil
}

func (s *PostService) GetByID(ctx context.Context, id int) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	return post, nil
}

func (s *PostService) GetAll(ctx context.Context, limit, offset int) ([]*model.Post, PaginationResult, error) {
	limit, offset = normalizePagination(limit, offset)

	posts, err := s.postRepo.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to get posts: %w", err)
	}

	total, err := s.postRepo.GetTotalCount(ctx)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to count posts: %w", err)
	}

	return posts, PaginationResult{Limit: limit, Offset: offset, Total: total}, nil
}

func (s *PostService) Update(ctx context.Context, id int, userID int, req *model.PostUpdateRequest) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	if !post.CanBeEditedBy(userID) {
		return nil, ErrForbidden
	}

	if err := validatePostUpdateRequest(req); err != nil {
		return nil, err
	}

	post.Title = strings.TrimSpace(req.Title)
	post.Content = strings.TrimSpace(req.Content)

	if err := s.postRepo.Update(ctx, post); err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	return post, nil
}

func (s *PostService) Delete(ctx context.Context, id int, userID int) error {
	post, err := s.postRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrPostNotFound
		}
		return fmt.Errorf("failed to get post: %w", err)
	}

	if !post.CanBeDeletedBy(userID) {
		return ErrForbidden
	}

	if err := s.postRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrPostNotFound
		}
		return fmt.Errorf("failed to delete post: %w", err)
	}

	return nil
}

func (s *PostService) GetByAuthor(ctx context.Context, authorID int, limit, offset int) ([]*model.Post, PaginationResult, error) {
	limit, offset = normalizePagination(limit, offset)

	posts, err := s.postRepo.GetByAuthorID(ctx, authorID, limit, offset)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to get posts by author: %w", err)
	}

	total, err := s.postRepo.GetTotalCountByAuthorID(ctx, authorID)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to count posts by author: %w", err)
	}

	return posts, PaginationResult{Limit: limit, Offset: offset, Total: total}, nil
}

// validatePostCreateRequest считает длину заголовка в символах, а не в байтах:
// иначе кириллица упиралась бы в лимит вдвое раньше
func validatePostCreateRequest(req *model.PostCreateRequest) error {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if utf8.RuneCountInString(title) > 200 {
		return fmt.Errorf("%w: title must be at most 200 characters long", ErrValidation)
	}
	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("%w: content is required", ErrValidation)
	}
	return nil
}

// validatePostUpdateRequest считает длину в символах по той же причине, что и validatePostCreateRequest
func validatePostUpdateRequest(req *model.PostUpdateRequest) error {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if utf8.RuneCountInString(title) > 200 {
		return fmt.Errorf("%w: title must be at most 200 characters long", ErrValidation)
	}
	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("%w: content is required", ErrValidation)
	}
	return nil
}
