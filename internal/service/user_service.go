package service

import (
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/pkg/auth"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

// emailRegexp — простая проверка формата email
// Библиотеки валидации в go.mod нет, поэтому теги validate в моделях служат
// документацией, а проверки написаны вручную в этом файле
var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type UserService struct {
	userRepo   repository.UserRepository
	jwtManager *auth.JWTManager
}

func NewUserService(userRepo repository.UserRepository, jwtManager *auth.JWTManager) *UserService {
	return &UserService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

func (s *UserService) Register(ctx context.Context, req *model.UserCreateRequest) (*model.TokenResponse, error) {
	if err := validateUserCreateRequest(req); err != nil {
		return nil, err
	}

	emailExists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if emailExists {
		return nil, ErrUserAlreadyExists
	}

	// В БД пишем username без пробелов по краям: валидация проверяла именно его
	username := strings.TrimSpace(req.Username)

	usernameExists, err := s.userRepo.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username existence: %w", err)
	}
	if usernameExists {
		return nil, ErrUserAlreadyExists
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Username: username,
		Email:    req.Email,
		Password: hashedPassword,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// Параллельная регистрация могла обогнать проверки Exists выше:
		// unique violation из БД — тот же конфликт, а не 500
		if errors.Is(err, repository.ErrUserExists) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	token, expiresAt, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user.ToResponse(),
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *model.UserLoginRequest) (*model.TokenResponse, error) {
	if err := validateUserLoginRequest(req); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// Не различаем "email не найден" и "пароль неверный": иначе перебором
		// можно узнать, какие email зарегистрированы (email enumeration)
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user.ToResponse(),
	}, nil
}

func (s *UserService) GetByID(ctx context.Context, id int) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// validateUserCreateRequest считает длину username в символах, а не в байтах:
// иначе кириллица упиралась бы в лимит вдвое раньше
func validateUserCreateRequest(req *model.UserCreateRequest) error {
	username := strings.TrimSpace(req.Username)
	if utf8.RuneCountInString(username) < 3 {
		return fmt.Errorf("%w: username must be at least 3 characters long", ErrValidation)
	}
	if utf8.RuneCountInString(username) > 50 {
		return fmt.Errorf("%w: username must be at most 50 characters long", ErrValidation)
	}

	if !emailRegexp.MatchString(req.Email) {
		return fmt.Errorf("%w: invalid email format", ErrValidation)
	}

	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}

	return nil
}

func validateUserLoginRequest(req *model.UserLoginRequest) error {
	if !emailRegexp.MatchString(req.Email) {
		return fmt.Errorf("%w: invalid email format", ErrValidation)
	}
	if req.Password == "" {
		return fmt.Errorf("%w: password is required", ErrValidation)
	}
	return nil
}
