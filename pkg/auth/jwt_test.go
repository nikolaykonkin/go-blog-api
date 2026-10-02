package auth

import (
	"errors"
	"testing"
	"time"
)

const testSecret = "test-secret-key-for-unit-tests-only"

func TestJWTManager_GenerateAndValidateToken(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)

	token, expiresAt, err := manager.GenerateToken(42, "user@example.com", "someuser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("expected expiresAt to be in the future")
	}

	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", claims.UserID)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", claims.Email)
	}
	if claims.Username != "someuser" {
		t.Errorf("expected username someuser, got %s", claims.Username)
	}
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	// Отрицательный TTL дает токен, истекший в момент создания, поэтому проверка
	// детерминированно возвращает ErrExpiredToken без time.Sleep
	manager := NewJWTManager(testSecret, -1)

	token, _, err := manager.GenerateToken(1, "expired@example.com", "expireduser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	_, err = manager.ValidateToken(token)
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWTManager_ValidateToken_InvalidSignature(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)
	otherManager := NewJWTManager("a-completely-different-secret-key", 1)

	token, _, err := otherManager.GenerateToken(1, "user@example.com", "someuser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	// Токен подписан другим секретом: ожидаем ErrInvalidToken, а не ErrExpiredToken
	_, err = manager.ValidateToken(token)
	if err == nil {
		t.Fatal("expected an error validating a token signed with a different secret")
	}
	if errors.Is(err, ErrExpiredToken) {
		t.Fatal("expected ErrInvalidToken, not ErrExpiredToken")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTManager_ValidateToken_Malformed(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)

	_, err := manager.ValidateToken("this-is-not-a-jwt")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for malformed token, got %v", err)
	}
}

func TestJWTManager_ValidateToken_Empty(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)

	_, err := manager.ValidateToken("")
	if err == nil {
		t.Fatal("expected an error validating an empty token string")
	}
}

func TestJWTManager_RefreshToken(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)

	original, _, err := manager.GenerateToken(7, "refresh@example.com", "refreshuser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	refreshed, newExpiresAt, err := manager.RefreshToken(original)
	if err != nil {
		t.Fatalf("unexpected error refreshing token: %v", err)
	}
	if refreshed == "" {
		t.Fatal("expected non-empty refreshed token")
	}
	if !newExpiresAt.After(time.Now()) {
		t.Fatal("expected refreshed token's expiry to be in the future")
	}

	claims, err := manager.ValidateToken(refreshed)
	if err != nil {
		t.Fatalf("unexpected error validating refreshed token: %v", err)
	}
	if claims.UserID != 7 || claims.Email != "refresh@example.com" || claims.Username != "refreshuser" {
		t.Fatalf("refreshed token claims do not match original: %+v", claims)
	}
}

func TestJWTManager_RefreshToken_FailsForExpiredToken(t *testing.T) {
	manager := NewJWTManager(testSecret, -1)

	token, _, err := manager.GenerateToken(1, "user@example.com", "someuser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	_, _, err = manager.RefreshToken(token)
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWTManager_GetUserIDFromToken(t *testing.T) {
	manager := NewJWTManager(testSecret, 1)

	token, _, err := manager.GenerateToken(99, "id@example.com", "iduser")
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	userID, err := manager.GetUserIDFromToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 99 {
		t.Fatalf("expected userID 99, got %d", userID)
	}

	t.Run("invalid token returns error", func(t *testing.T) {
		if _, err := manager.GetUserIDFromToken("garbage"); err == nil {
			t.Fatal("expected an error for an invalid token")
		}
	})
}
