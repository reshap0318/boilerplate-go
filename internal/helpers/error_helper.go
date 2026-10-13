package helpers

import "errors"

// FieldError represents an error tied to a specific field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return e.Message
}

type CustomError struct {
	Status  int
	Code    string
	Message string
}

func (e *CustomError) Error() string {
	return e.Message
}

// Common application errors.
var (
	ErrNotFound          = errors.New("record not found")
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token expired")
	ErrInvalidCredential = errors.New("invalid email or password")
	ErrTokenExpired      = errors.New("reset token has expired")
	ErrTokenUsed         = errors.New("reset token has already been used")
	ErrTokenInvalid      = errors.New("invalid reset token")
	ErrForbidden         = errors.New("forbidden")
)
