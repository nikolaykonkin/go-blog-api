package handler

import (
	"blog-api/internal/model"
	"blog-api/internal/service"
	"blog-api/pkg/auth"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestJWTManagerForHandlers() *auth.JWTManager {
	return auth.NewJWTManager("handler-test-secret-key-1234567890", 24)
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("success returns 201 with a token", func(t *testing.T) {
		userRepo := &mockUserRepo{
			createFunc: func(ctx context.Context, user *model.User) error {
				user.ID = 1
				return nil
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		body := `{"username":"newuser","email":"new@example.com","password":"password1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp model.TokenResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Token == "" {
			t.Fatal("expected a non-empty token in the response")
		}
		if resp.User.Email != "new@example.com" {
			t.Fatalf("unexpected user in response: %+v", resp.User)
		}
	})

	t.Run("malformed JSON body returns 400", func(t *testing.T) {
		userService := service.NewUserService(&mockUserRepo{}, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader("{not valid json"))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("weak password returns 400", func(t *testing.T) {
		userService := service.NewUserService(&mockUserRepo{}, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		body := `{"username":"newuser","email":"new@example.com","password":"a"}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		userRepo := &mockUserRepo{
			existsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
				return true, nil
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		body := `{"username":"newuser","email":"taken@example.com","password":"password1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	hashed, err := auth.HashPassword("correct-password1")
	if err != nil {
		t.Fatalf("failed to hash fixture password: %v", err)
	}
	existingUser := &model.User{ID: 1, Username: "loginuser", Email: "login@example.com", Password: hashed}

	t.Run("success returns 200 with a token", func(t *testing.T) {
		userRepo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return existingUser, nil
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		body := `{"email":"login@example.com","password":"correct-password1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		userRepo := &mockUserRepo{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return existingUser, nil
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		body := `{"email":"login@example.com","password":"wrong-password"}`
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed JSON body returns 400", func(t *testing.T) {
		userService := service.NewUserService(&mockUserRepo{}, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("not json"))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestAuthHandler_GetProfile(t *testing.T) {
	t.Run("authenticated user gets their profile", func(t *testing.T) {
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return &model.User{ID: id, Username: "profileuser", Email: "profile@example.com"}, nil
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		rec := httptest.NewRecorder()

		withUser(3)(h.GetProfile)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp model.UserResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ID != 3 || resp.Username != "profileuser" {
			t.Fatalf("unexpected profile response: %+v", resp)
		}
	})

	t.Run("missing auth context returns 401", func(t *testing.T) {
		userService := service.NewUserService(&mockUserRepo{}, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		rec := httptest.NewRecorder()

		h.GetProfile(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("user no longer exists returns 404", func(t *testing.T) {
		userRepo := &mockUserRepo{
			getByIDFunc: func(ctx context.Context, id int) (*model.User, error) {
				return nil, service.ErrUserNotFound
			},
		}
		userService := service.NewUserService(userRepo, newTestJWTManagerForHandlers())
		h := NewAuthHandler(userService)

		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		rec := httptest.NewRecorder()

		withUser(999)(h.GetProfile)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestRegisterRequestBodyIsValidJSON(t *testing.T) {
	body := []byte(`{"username":"newuser","email":"new@example.com","password":"password1"}`)
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		t.Fatalf("test fixture body is not valid JSON: %v", err)
	}
}
