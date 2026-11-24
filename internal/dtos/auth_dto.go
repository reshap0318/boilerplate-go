package dtos

import "time"

// LoginRequest represents the login request payload.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents the login response payload.
type LoginResponse struct {
	Token        string  `json:"token"`
	RefreshToken string  `json:"refresh_token"`
	User         UserDTO `json:"user"`
}

// RefreshTokenRequest represents the refresh token request payload.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest is the optional logout payload. When RefreshToken is set, it is
// revoked together with the access token from the Authorization header.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RegisterRequest represents the register request payload.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Name     string `json:"name" validate:"required,min=3"`
}

// UserDTO represents user data transfer object.
type UserDTO struct {
	ID          uint            `json:"id"`
	Email       string          `json:"email"`
	Name        string          `json:"name"`
	Avatar      string          `json:"avatar"`
	CreatedAt   time.Time       `json:"created_at"`
	Roles       []RoleMiniDTO   `json:"roles"`
	Permissions []PermissionDTO `json:"permissions"`
}

// ForgetPasswordRequest represents the forget password request payload.
type ForgetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents the reset password request payload.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// ResendVerificationRequest represents the resend verification email request payload.
type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// VerifyEmailRequest represents the verify email request payload.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// MessageResponse represents a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// UploadFileResponse represents the file upload response payload.
type UploadFileResponse struct {
	UUID string `json:"uuid"`
	URL  string `json:"url"`
}
