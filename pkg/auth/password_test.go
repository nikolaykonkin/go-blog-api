package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	t.Run("empty password returns error", func(t *testing.T) {
		_, err := HashPassword("")
		if !errors.Is(err, ErrEmptyPassword) {
			t.Fatalf("expected ErrEmptyPassword, got %v", err)
		}
	})

	t.Run("valid password produces a bcrypt hash", func(t *testing.T) {
		hash, err := HashPassword("supersecret1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash")
		}
		if hash == "supersecret1" {
			t.Fatal("hash must not equal the plaintext password")
		}
		// bcrypt-хеши всегда начинаются с одного из этих префиксов версии
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") && !strings.HasPrefix(hash, "$2y$") {
			t.Fatalf("hash does not look like a bcrypt hash: %s", hash)
		}
	})

	t.Run("hashing the same password twice yields different hashes (salted)", func(t *testing.T) {
		h1, err := HashPassword("samepassword1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		h2, err := HashPassword("samepassword1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h1 == h2 {
			t.Fatal("expected different hashes due to bcrypt salting")
		}
	})
}

func TestCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-password1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("correct password matches", func(t *testing.T) {
		if !CheckPassword("correct-password1", hash) {
			t.Fatal("expected password to match its own hash")
		}
	})

	t.Run("incorrect password does not match", func(t *testing.T) {
		if CheckPassword("wrong-password1", hash) {
			t.Fatal("expected mismatched password to fail")
		}
	})

	t.Run("malformed hash does not match", func(t *testing.T) {
		if CheckPassword("correct-password1", "not-a-real-hash") {
			t.Fatal("expected malformed hash to fail")
		}
	})
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		expectErr bool
	}{
		{"empty password", "", true},
		{"too short", "ab1", true},
		{"only letters", "abcdefgh", true},
		{"only digits", "12345678", true},
		{"letters and digits, min length", "abc123", false},
		{"letters and digits, longer", "Password123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if tt.expectErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}

	t.Run("empty password returns ErrEmptyPassword specifically", func(t *testing.T) {
		if err := ValidatePasswordStrength(""); !errors.Is(err, ErrEmptyPassword) {
			t.Fatalf("expected ErrEmptyPassword, got %v", err)
		}
	})

	t.Run("short password returns ErrPasswordTooShort specifically", func(t *testing.T) {
		if err := ValidatePasswordStrength("a1"); !errors.Is(err, ErrPasswordTooShort) {
			t.Fatalf("expected ErrPasswordTooShort, got %v", err)
		}
	})
}

func TestGenerateRandomPassword(t *testing.T) {
	t.Run("non-positive length returns error", func(t *testing.T) {
		if _, err := GenerateRandomPassword(0); err == nil {
			t.Fatal("expected error for zero length")
		}
		if _, err := GenerateRandomPassword(-5); err == nil {
			t.Fatal("expected error for negative length")
		}
	})

	t.Run("produces a password of the requested length", func(t *testing.T) {
		pw, err := GenerateRandomPassword(16)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pw) != 16 {
			t.Fatalf("expected length 16, got %d", len(pw))
		}
	})

	t.Run("only uses characters from the expected charset", func(t *testing.T) {
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		pw, err := GenerateRandomPassword(64)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, ch := range pw {
			if !strings.ContainsRune(charset, ch) {
				t.Fatalf("unexpected character %q in generated password", ch)
			}
		}
	})

	t.Run("consecutive calls are not identical", func(t *testing.T) {
		pw1, _ := GenerateRandomPassword(32)
		pw2, _ := GenerateRandomPassword(32)
		if pw1 == pw2 {
			t.Fatal("expected different random passwords on consecutive calls")
		}
	})
}
