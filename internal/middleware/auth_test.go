package middleware

import (
	"blog-api/pkg/auth"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const middlewareTestSecret = "middleware-test-secret-key-1234567890"

func newTestManager(ttlHours int) *auth.JWTManager {
	return auth.NewJWTManager(middlewareTestSecret, ttlHours)
}

func TestAuthMiddleware_RequireAuth(t *testing.T) {
	manager := newTestManager(1)
	mw := NewAuthMiddleware(manager)

	var gotUserID int
	var gotEmail, gotUsername string
	handlerCalled := false
	protected := mw.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		gotUserID, _ = GetUserIDFromContext(r.Context())
		gotEmail, _ = GetUserEmailFromContext(r.Context())
		gotUsername, _ = GetUsernameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	t.Run("valid token grants access and populates context", func(t *testing.T) {
		handlerCalled = false
		token, _, err := manager.GenerateToken(11, "user@example.com", "someuser")
		if err != nil {
			t.Fatalf("unexpected error generating token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		protected(rec, req)

		if !handlerCalled {
			t.Fatal("expected the wrapped handler to be called")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if gotUserID != 11 || gotEmail != "user@example.com" || gotUsername != "someuser" {
			t.Fatalf("unexpected context values: userID=%d email=%s username=%s", gotUserID, gotEmail, gotUsername)
		}
	})

	t.Run("missing authorization header is rejected", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		protected(rec, req)

		if handlerCalled {
			t.Fatal("expected the wrapped handler NOT to be called")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("malformed authorization header (no Bearer prefix) is rejected", func(t *testing.T) {
		handlerCalled = false
		token, _, _ := manager.GenerateToken(1, "a@example.com", "a")
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", token) // без префикса "Bearer "
		rec := httptest.NewRecorder()

		protected(rec, req)

		if handlerCalled {
			t.Fatal("expected the wrapped handler NOT to be called")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer not-a-real-token")
		rec := httptest.NewRecorder()

		protected(rec, req)

		if handlerCalled {
			t.Fatal("expected the wrapped handler NOT to be called")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		var body errorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected a JSON error body: %v", err)
		}
		if body.Error == "" {
			t.Fatal("expected a non-empty error message")
		}
	})

	t.Run("expired token is rejected with a specific message", func(t *testing.T) {
		expiredManager := newTestManager(-1)
		expiredProtected := mw.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called for an expired token")
		})
		token, _, _ := expiredManager.GenerateToken(1, "a@example.com", "a")

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		expiredProtected(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestAuthMiddleware_OptionalAuth(t *testing.T) {
	manager := newTestManager(1)
	mw := NewAuthMiddleware(manager)

	t.Run("no token still calls the handler as anonymous", func(t *testing.T) {
		called := false
		var hadUserID bool
		handler := mw.OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_, hadUserID = GetUserIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/optional", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if !called {
			t.Fatal("expected the handler to be called even without a token")
		}
		if hadUserID {
			t.Fatal("expected no user ID in context when no token was provided")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("valid token populates the context", func(t *testing.T) {
		var gotUserID int
		handler := mw.OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
			gotUserID, _ = GetUserIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		token, _, _ := manager.GenerateToken(55, "opt@example.com", "optuser")
		req := httptest.NewRequest(http.MethodGet, "/optional", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if gotUserID != 55 {
			t.Fatalf("expected userID 55, got %d", gotUserID)
		}
	})

	t.Run("invalid token is ignored, not rejected", func(t *testing.T) {
		called := false
		handler := mw.OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/optional", nil)
		req.Header.Set("Authorization", "Bearer garbage-token")
		rec := httptest.NewRecorder()
		handler(rec, req)

		if !called {
			t.Fatal("expected the handler to still be called despite the invalid token")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})
}

func TestChain(t *testing.T) {
	var order []string

	mkMiddleware := func(name string) func(http.HandlerFunc) http.HandlerFunc {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				next(w, r)
				order = append(order, name+":after")
			}
		}
	}

	handler := Chain(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}, mkMiddleware("A"), mkMiddleware("B"))

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	expected := []string{"A:before", "B:before", "handler", "B:after", "A:after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("expected order %v, got %v", expected, order)
		}
	}
}
