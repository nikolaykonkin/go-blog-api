package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmptyPassword    = errors.New("password cannot be empty")
	ErrPasswordTooShort = errors.New("password is too short")
)

// bcryptCost = 12 — компромисс между стойкостью к перебору и временем ответа
// на Login/Register: bcrypt медленный намеренно
const bcryptCost = 12

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePasswordStrength требует длину не меньше 6, как в теге validate у
// UserCreateRequest.Password, и хотя бы одну букву и одну цифру
func ValidatePasswordStrength(password string) error {
	if password == "" {
		return ErrEmptyPassword
	}

	if len(password) < 6 {
		return ErrPasswordTooShort
	}

	var hasLetter, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsLetter(ch):
			hasLetter = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return errors.New("password should contain at least one letter and one digit")
	}

	return nil
}

func GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("password length must be positive")
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result, err := randomStringFromCharset(length, charset)
	if err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}

	return result, nil
}

// randomStringFromCharset берет crypto/rand, а не math/rand: для пароля нужна непредсказуемость
func randomStringFromCharset(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range result {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}

	return string(result), nil
}
