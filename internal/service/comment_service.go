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
	ErrCommentNotFound = errors.New("comment not found")
	ErrPostNotExists   = errors.New("post does not exist")
)

type CommentService struct {
	commentRepo repository.CommentRepository
	postRepo    repository.PostRepository
	userRepo    repository.UserRepository
}

func NewCommentService(
	commentRepo repository.CommentRepository,
	postRepo repository.PostRepository,
	userRepo repository.UserRepository,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
		userRepo:    userRepo,
	}
}

func (s *CommentService) Create(ctx context.Context, userID int, req *model.CommentCreateRequest) (*model.Comment, error) {
	if err := validateCommentCreateRequest(req); err != nil {
		return nil, err
	}

	exists, err := s.postRepo.Exists(ctx, req.PostID)
	if err != nil {
		return nil, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return nil, ErrPostNotExists
	}

	// Сохраняем текст без пробелов по краям: валидация проверяла именно его
	comment := &model.Comment{
		Content:  strings.TrimSpace(req.Content),
		PostID:   req.PostID,
		AuthorID: userID,
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

func (s *CommentService) GetByID(ctx context.Context, id int) (*model.Comment, error) {
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	return comment, nil
}

func (s *CommentService) GetByPost(ctx context.Context, postID int, limit, offset int) ([]*model.Comment, PaginationResult, error) {
	const defaultCommentLimit = 20
	if limit <= 0 {
		limit = defaultCommentLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}

	exists, err := s.postRepo.Exists(ctx, postID)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return nil, PaginationResult{}, ErrPostNotExists
	}

	comments, err := s.commentRepo.GetByPostID(ctx, postID, limit, offset)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to get comments: %w", err)
	}

	total, err := s.commentRepo.GetCountByPostID(ctx, postID)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to count comments: %w", err)
	}

	return comments, PaginationResult{Limit: limit, Offset: offset, Total: total}, nil
}

func (s *CommentService) Update(ctx context.Context, id int, userID int, req *model.CommentUpdateRequest) (*model.Comment, error) {
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	if !comment.CanBeEditedBy(userID) {
		return nil, ErrForbidden
	}

	if err := validateCommentUpdateRequest(req); err != nil {
		return nil, err
	}

	comment.Content = strings.TrimSpace(req.Content)

	if err := s.commentRepo.Update(ctx, comment); err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	return comment, nil
}

func (s *CommentService) Delete(ctx context.Context, id int, userID int) error {
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return ErrCommentNotFound
		}
		return fmt.Errorf("failed to get comment: %w", err)
	}

	if !comment.CanBeDeletedBy(userID) {
		return ErrForbidden
	}

	if err := s.commentRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return ErrCommentNotFound
		}
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

func (s *CommentService) GetByAuthor(ctx context.Context, authorID int, limit, offset int) ([]*model.Comment, PaginationResult, error) {
	limit, offset = normalizePagination(limit, offset)

	comments, err := s.commentRepo.GetByAuthorID(ctx, authorID, limit, offset)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to get comments by author: %w", err)
	}

	total, err := s.commentRepo.GetTotalCountByAuthorID(ctx, authorID)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("failed to count comments by author: %w", err)
	}

	return comments, PaginationResult{Limit: limit, Offset: offset, Total: total}, nil
}

// validateCommentCreateRequest считает длину в символах, а не в байтах:
// иначе кириллица упиралась бы в лимит вдвое раньше
func validateCommentCreateRequest(req *model.CommentCreateRequest) error {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return fmt.Errorf("%w: content is required", ErrValidation)
	}
	if utf8.RuneCountInString(content) > 1000 {
		return fmt.Errorf("%w: content must be at most 1000 characters long", ErrValidation)
	}
	if req.PostID <= 0 {
		return fmt.Errorf("%w: valid post id is required", ErrValidation)
	}
	return nil
}

// validateCommentUpdateRequest считает длину в символах по той же причине, что и validateCommentCreateRequest
func validateCommentUpdateRequest(req *model.CommentUpdateRequest) error {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return fmt.Errorf("%w: content is required", ErrValidation)
	}
	if utf8.RuneCountInString(content) > 1000 {
		return fmt.Errorf("%w: content must be at most 1000 characters long", ErrValidation)
	}
	return nil
}
