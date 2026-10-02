package service

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/pkg/auth"
	"context"
	"errors"
	"testing"
)

func newTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager("test-secret-key-at-least-32-chars-long", 24)
}

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("success creates the user and returns a token", func(t *testing.T) {
		var created *model.User
		repo := &mockUserRepo{
			createFunc: func(ctx context.Context, user *model.User) error {
				user.ID = 100
				created = user
				return nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		resp, err := svc.Register(ctx, &model.UserCreateRequest{
			Username: "newuser",
			Email:    "newuser@example.com",
			Password: "password1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Token == "" {
			t.Fatal("expected a non-empty token")
		}
		if resp.User.Username != "newuser" || resp.User.Email != "newuser@example.com" {
			t.Fatalf("unexpected user in response: %+v", resp.User)
		}
		if created == nil {
			t.Fatal("expected repo.Create to have been called")
		}
		if created.Password == "password1" {
			t.Fatal("expected the password to be hashed before storage")
		}
		if !auth.CheckPassword("password1", created.Password) {
			t.Fatal("stored hash does not match the original password")
		}
	})

	t.Run("email already exists", func(t *testing.T) {
		repo := &mockUserRepo{
			existsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
				return true, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Register(ctx, &model.UserCreateRequest{
			Username: "someuser",
			Email:    "taken@example.com",
			Password: "password1",
		})
		if !errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})

	t.Run("username already exists", func(t *testing.T) {
		repo := &mockUserRepo{
			existsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
				return false, nil
			},
			existsByUsernameFunc: func(ctx context.Context, username string) (bool, error) {
				return true, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Register(ctx, &model.UserCreateRequest{
			Username: "takenuser",
			Email:    "fresh@example.com",
			Password: "password1",
		})
		if !errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})

	validationCases := []struct {
		name string
		req  model.UserCreateRequest
	}{
		{"username too short", model.UserCreateRequest{Username: "ab", Email: "a@example.com", Password: "password1"}},
		{"invalid email", model.UserCreateRequest{Username: "validuser", Email: "not-an-email", Password: "password1"}},
		{"password too short", model.UserCreateRequest{Username: "validuser", Email: "a@example.com", Password: "p1"}},
		{"password missing digit", model.UserCreateRequest{Username: "validuser", Email: "a@example.com", Password: "onlyletters"}},
	}

	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockUserRepo{}
			svc := NewUserService(repo, newTestJWTManager())

			req := tc.req
			_, err := svc.Register(ctx, &req)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}

	t.Run("repository create failure propagates as a generic error", func(t *testing.T) {
		repo := &mockUserRepo{
			createFunc: func(ctx context.Context, user *model.User) error {
				return errors.New("boom: db is down")
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Register(ctx, &model.UserCreateRequest{
			Username: "validuser",
			Email:    "valid@example.com",
			Password: "password1",
		})
		if err == nil {
			t.Fatal("expected an error")
		}
		if errors.Is(err, ErrValidation) || errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("did not expect a sentinel service error, got %v", err)
		}
	})
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()

	hashed, err := auth.HashPassword("correct-password1")
	if err != nil {
		t.Fatalf("failed to hash password for fixture: %v", err)
	}
	existingUser := &model.User{
		ID:       5,
		Username: "loginuser",
		Email:    "login@example.com",
		Password: hashed,
	}

	t.Run("success returns a token for correct credentials", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				if email == existingUser.Email {
					return existingUser, nil
				}
				return nil, repository.ErrUserNotFound
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		resp, err := svc.Login(ctx, &model.UserLoginRequest{
			Email:    "login@example.com",
			Password: "correct-password1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Token == "" {
			t.Fatal("expected a non-empty token")
		}
		if resp.User.ID != existingUser.ID {
			t.Fatalf("expected user ID %d, got %d", existingUser.ID, resp.User.ID)
		}
	})

	t.Run("unknown email returns invalid credentials, not user-not-found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Login(ctx, &model.UserLoginRequest{
			Email:    "nobody@example.com",
			Password: "whatever1",
		})
		// Сервис намеренно сводит "пользователь не найден" и "неверный пароль"
		// к одной ошибке, чтобы нельзя было перебрать email
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("wrong password returns invalid credentials", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return existingUser, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Login(ctx, &model.UserLoginRequest{
			Email:    "login@example.com",
			Password: "totally-wrong-password",
		})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("invalid email format is rejected before hitting the repository", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				t.Fatal("repository should not be called for an invalid email format")
				return nil, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Login(ctx, &model.UserLoginRequest{
			Email:    "not-an-email",
			Password: "whatever1",
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("empty password is rejected", func(t *testing.T) {
		repo := &mockUserRepo{}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.Login(ctx, &model.UserLoginRequest{
			Email:    "login@example.com",
			Password: "",
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
}

func TestUserService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id, Username: "found"}, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		user, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Username != "found" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.GetByID(ctx, 999)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUserService_GetByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return &model.User{ID: 3, Email: email}, nil
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		user, err := svc.GetByEmail(ctx, "someone@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "someone@example.com" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}
		svc := NewUserService(repo, newTestJWTManager())

		_, err := svc.GetByEmail(ctx, "nobody@example.com")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}
