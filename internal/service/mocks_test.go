package service

import (
	"blog-api/internal/model"
	"context"
)

// mockUserRepo — настраиваемая заглушка repository.UserRepository
// Каждый метод вызывает необязательное поле-функцию, поэтому тест переопределяет
// только нужное поведение, а остальное возвращает значения по умолчанию
type mockUserRepo struct {
	createFunc           func(ctx context.Context, user *model.User) error
	getByIDFunc          func(ctx context.Context, id int) (*model.User, error)
	getByEmailFunc       func(ctx context.Context, email string) (*model.User, error)
	getByUsernameFunc    func(ctx context.Context, username string) (*model.User, error)
	existsByEmailFunc    func(ctx context.Context, email string) (bool, error)
	existsByUsernameFunc func(ctx context.Context, username string) (bool, error)
	updateFunc           func(ctx context.Context, user *model.User) error
	deleteFunc           func(ctx context.Context, id int) error
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, user)
	}
	user.ID = 1
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int) (*model.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.getByEmailFunc != nil {
		return m.getByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if m.getByUsernameFunc != nil {
		return m.getByUsernameFunc(ctx, username)
	}
	return nil, nil
}

func (m *mockUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if m.existsByEmailFunc != nil {
		return m.existsByEmailFunc(ctx, email)
	}
	return false, nil
}

func (m *mockUserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	if m.existsByUsernameFunc != nil {
		return m.existsByUsernameFunc(ctx, username)
	}
	return false, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *model.User) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

// mockPostRepo — настраиваемая заглушка repository.PostRepository, устроена как mockUserRepo
type mockPostRepo struct {
	createFunc                  func(ctx context.Context, post *model.Post) error
	getByIDFunc                 func(ctx context.Context, id int) (*model.Post, error)
	getAllFunc                  func(ctx context.Context, limit, offset int) ([]*model.Post, error)
	getTotalCountFunc           func(ctx context.Context) (int, error)
	updateFunc                  func(ctx context.Context, post *model.Post) error
	deleteFunc                  func(ctx context.Context, id int) error
	existsFunc                  func(ctx context.Context, id int) (bool, error)
	getByAuthorIDFunc           func(ctx context.Context, authorID int, limit, offset int) ([]*model.Post, error)
	getTotalCountByAuthorIDFunc func(ctx context.Context, authorID int) (int, error)

	// capturedLimit и capturedOffset запоминают аргументы, с которыми сервис вызвал
	// репозиторий, чтобы тесты проверяли нормализацию пагинации
	capturedLimit  int
	capturedOffset int
}

func (m *mockPostRepo) Create(ctx context.Context, post *model.Post) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, post)
	}
	post.ID = 1
	return nil
}

func (m *mockPostRepo) GetByID(ctx context.Context, id int) (*model.Post, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockPostRepo) GetAll(ctx context.Context, limit, offset int) ([]*model.Post, error) {
	m.capturedLimit, m.capturedOffset = limit, offset
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx, limit, offset)
	}
	return nil, nil
}

func (m *mockPostRepo) GetTotalCount(ctx context.Context) (int, error) {
	if m.getTotalCountFunc != nil {
		return m.getTotalCountFunc(ctx)
	}
	return 0, nil
}

func (m *mockPostRepo) Update(ctx context.Context, post *model.Post) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, post)
	}
	return nil
}

func (m *mockPostRepo) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockPostRepo) Exists(ctx context.Context, id int) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, id)
	}
	return true, nil
}

func (m *mockPostRepo) GetByAuthorID(ctx context.Context, authorID int, limit, offset int) ([]*model.Post, error) {
	m.capturedLimit, m.capturedOffset = limit, offset
	if m.getByAuthorIDFunc != nil {
		return m.getByAuthorIDFunc(ctx, authorID, limit, offset)
	}
	return nil, nil
}

func (m *mockPostRepo) GetTotalCountByAuthorID(ctx context.Context, authorID int) (int, error) {
	if m.getTotalCountByAuthorIDFunc != nil {
		return m.getTotalCountByAuthorIDFunc(ctx, authorID)
	}
	return 0, nil
}

// mockCommentRepo — настраиваемая заглушка repository.CommentRepository, устроена как mockUserRepo
type mockCommentRepo struct {
	createFunc                  func(ctx context.Context, comment *model.Comment) error
	getByIDFunc                 func(ctx context.Context, id int) (*model.Comment, error)
	getByPostIDFunc             func(ctx context.Context, postID int, limit, offset int) ([]*model.Comment, error)
	getCountByPostIDFunc        func(ctx context.Context, postID int) (int, error)
	updateFunc                  func(ctx context.Context, comment *model.Comment) error
	deleteFunc                  func(ctx context.Context, id int) error
	getByAuthorIDFunc           func(ctx context.Context, authorID int, limit, offset int) ([]*model.Comment, error)
	getTotalCountByAuthorIDFunc func(ctx context.Context, authorID int) (int, error)

	capturedLimit  int
	capturedOffset int
}

func (m *mockCommentRepo) Create(ctx context.Context, comment *model.Comment) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, comment)
	}
	comment.ID = 1
	return nil
}

func (m *mockCommentRepo) GetByID(ctx context.Context, id int) (*model.Comment, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockCommentRepo) GetByPostID(ctx context.Context, postID int, limit, offset int) ([]*model.Comment, error) {
	m.capturedLimit, m.capturedOffset = limit, offset
	if m.getByPostIDFunc != nil {
		return m.getByPostIDFunc(ctx, postID, limit, offset)
	}
	return nil, nil
}

func (m *mockCommentRepo) GetCountByPostID(ctx context.Context, postID int) (int, error) {
	if m.getCountByPostIDFunc != nil {
		return m.getCountByPostIDFunc(ctx, postID)
	}
	return 0, nil
}

func (m *mockCommentRepo) Update(ctx context.Context, comment *model.Comment) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, comment)
	}
	return nil
}

func (m *mockCommentRepo) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockCommentRepo) GetByAuthorID(ctx context.Context, authorID int, limit, offset int) ([]*model.Comment, error) {
	m.capturedLimit, m.capturedOffset = limit, offset
	if m.getByAuthorIDFunc != nil {
		return m.getByAuthorIDFunc(ctx, authorID, limit, offset)
	}
	return nil, nil
}

func (m *mockCommentRepo) GetTotalCountByAuthorID(ctx context.Context, authorID int) (int, error) {
	if m.getTotalCountByAuthorIDFunc != nil {
		return m.getTotalCountByAuthorIDFunc(ctx, authorID)
	}
	return 0, nil
}
