package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// parseToken verifies signature, expiry, token type and the logout blacklist.
// expectedType is helpers.TokenTypeAccess or helpers.TokenTypeRefresh — an access
// token is never accepted where a refresh token is expected, and vice versa.
func (s *Services) parseToken(tokenString, expectedType string) (*helpers.JWTClaims, error) {
	claims, err := helpers.ValidateToken(tokenString, s.JWKSManager.GetPublicKey())
	if err != nil {
		return nil, helpers.ErrInvalidToken
	}

	if claims.TokenType != expectedType {
		return nil, helpers.ErrInvalidToken
	}

	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		return nil, helpers.ErrExpiredToken
	}

	// Check token blacklist — reject if jti has been revoked (logout / refresh rotation)
	if s.RedisClient.IsCacheAvailable() {
		if blacklisted, err := s.RedisClient.Exists(blacklistKey(claims.ID)); err == nil && blacklisted {
			return nil, helpers.ErrInvalidToken
		}
	}

	return claims, nil
}

func blacklistKey(jti string) string {
	return fmt.Sprintf("blacklist:jti:%s", jti)
}

// blacklistToken revokes a token's jti until the token would have expired anyway.
func (s *Services) blacklistToken(claims *helpers.JWTClaims) {
	if claims == nil || claims.ExpiresAt == nil {
		return
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return
	}
	if !s.RedisClient.IsCacheAvailable() {
		s.Logger.LogWarn("blacklistToken", "Redis unavailable — jti %s not blacklisted", claims.ID)
		return
	}
	if err := s.RedisClient.Set(blacklistKey(claims.ID), "1", ttl); err != nil {
		s.Logger.LogWarn("blacklistToken", "Failed to blacklist jti %s: %v", claims.ID, err)
	}
}

// AuthValidateToken validates an ACCESS token and returns the claims.
func (s *Services) AuthValidateToken(tokenString string) (*helpers.JWTClaims, error) {
	claims, err := s.parseToken(tokenString, helpers.TokenTypeAccess)
	if err != nil {
		return nil, err
	}

	// Cached session means the user was active when it was written; Invalidate()
	// deletes it on user update/delete, so a hit is enough.
	if s.RedisClient.IsCacheAvailable() {
		var cachedUserDTO dtos.UserDTO
		if err := s.RedisClient.GetJSON(fmt.Sprintf("session:%d", claims.UserID), &cachedUserDTO); err == nil {
			return claims, nil
		}
	}

	// Fallback: validate user from database (primary-key lookup)
	exists, err := s.repo.User.ExistsWithMap(nil, map[string]interface{}{"id": claims.UserID})
	if err != nil || !exists {
		s.Logger.LogStep("AuthValidateToken", "User not found in DB: %d", claims.UserID)
		return nil, helpers.ErrInvalidCredential
	}

	// Re-populate the session cache so the next request does not hit the DB again.
	s.Access.Warm(claims.UserID)

	return claims, nil
}

// AuthLogin authenticates a user and returns tokens.
func (s *Services) AuthLogin(ctx context.Context, email, password string) (*dtos.LoginResponse, error) {
	s.Logger.LogStart("AuthLogin", "User login attempt: %s", email)

	// Single lookup with roles+permissions preloaded (no second fetch after password check)
	user, err := s.repo.User.FindByEmail(nil, email, "Roles.Permissions")
	if err != nil {
		s.Logger.LogEndWithError("AuthLogin", "Login failed - user not found: %s", email)
		return nil, helpers.ErrInvalidCredential
	}

	if !s.checkPassword(password, user.Password) {
		s.Logger.LogEndWithError("AuthLogin", "Login failed - invalid password: %s", email)
		return nil, helpers.ErrInvalidCredential
	}
	s.Logger.LogStep("AuthLogin", "Password validated successfully")

	token, refreshToken, err := s.issueTokens(user)
	if err != nil {
		s.Logger.LogEndWithError("AuthLogin", "Login failed - token generation error: %v", err)
		return nil, err
	}

	// Drop any stale access cache so login always reflects current roles/permissions.
	s.Access.Invalidate(user.ID)

	userDTO := dtos.ToUserDTO(user)

	// Cache UserDTO to Redis with fallback
	if s.RedisClient.IsCacheAvailable() {
		sessionKey := fmt.Sprintf("session:%d", user.ID)
		if err := s.RedisClient.SetJSON(sessionKey, userDTO, s.cfg.Expiration); err != nil {
			s.Logger.LogWarn("AuthLogin", "Failed to cache session to Redis: %v", err)
		}
	}

	s.Logger.LogEnd("AuthLogin", "Login successful for user: %s", email)
	return &dtos.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         userDTO,
	}, nil
}

// AuthRefreshToken exchanges a REFRESH token for a new token pair.
// The used refresh token is blacklisted (rotation), so it cannot be replayed.
func (s *Services) AuthRefreshToken(ctx context.Context, refreshToken string) (*dtos.LoginResponse, error) {
	s.Logger.LogStart("AuthRefreshToken", "Token refresh attempt")

	claims, err := s.parseToken(refreshToken, helpers.TokenTypeRefresh)
	if err != nil {
		s.Logger.LogEndWithError("AuthRefreshToken", "Token refresh failed - invalid refresh token: %v", err)
		return nil, err
	}

	user, err := s.repo.User.FindByID(nil, claims.UserID)
	if err != nil {
		s.Logger.LogEndWithError("AuthRefreshToken", "Token refresh failed - user not found: %v", err)
		return nil, helpers.ErrInvalidCredential
	}

	token, newRefreshToken, err := s.issueTokens(user)
	if err != nil {
		s.Logger.LogEndWithError("AuthRefreshToken", "Token refresh failed - token generation error: %v", err)
		return nil, err
	}

	// Rotate: the old refresh token is single-use.
	s.blacklistToken(claims)

	s.Logger.LogEnd("AuthRefreshToken", "Token refreshed successfully for user: %s", user.Email)
	return &dtos.LoginResponse{
		Token:        token,
		RefreshToken: newRefreshToken,
		User:         dtos.ToUserDTO(user),
	}, nil
}

// AuthLogout blacklists the access token's jti, and the refresh token's jti when
// one is supplied (it must belong to the same user), so neither can be reused.
func (s *Services) AuthLogout(ctx context.Context, tokenString, refreshToken string) error {
	s.Logger.LogStart("AuthLogout", "Logout request")

	claims, err := helpers.ValidateToken(tokenString, s.JWKSManager.GetPublicKey())
	if err != nil {
		s.Logger.LogEndWithError("AuthLogout", "Invalid token: %v", err)
		return helpers.ErrInvalidToken
	}
	s.blacklistToken(claims)

	if refreshToken != "" {
		refreshClaims, err := helpers.ValidateToken(refreshToken, s.JWKSManager.GetPublicKey())
		if err == nil && refreshClaims.TokenType == helpers.TokenTypeRefresh && refreshClaims.UserID == claims.UserID {
			s.blacklistToken(refreshClaims)
		} else {
			s.Logger.LogWarn("AuthLogout", "Refresh token ignored (invalid or not owned by caller)")
		}
	}

	s.Logger.LogEnd("AuthLogout", "Logout successful for user: %d", claims.UserID)
	return nil
}

// Helper functions

func (s *Services) checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// issueTokens signs a new access + refresh token pair for the user.
func (s *Services) issueTokens(user *models.User) (access, refresh string, err error) {
	key, kid := s.JWKSManager.GetPrivateKey(), s.JWKSManager.GetKeyID()
	access, err = helpers.GenerateToken(user.ID, user.Email, helpers.TokenTypeAccess, key, kid, helpers.GetEnvInt("JWT_EXPIRATION", 24))
	if err != nil {
		return "", "", err
	}
	refresh, err = helpers.GenerateToken(user.ID, user.Email, helpers.TokenTypeRefresh, key, kid, helpers.GetEnvInt("JWT_REFRESH_EXPIRATION", 168))
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// AuthForgetPassword generates a reset token and sends it via email.
// It returns nil for unknown emails as well, so the endpoint cannot be used to
// discover which emails are registered.
func (s *Services) AuthForgetPassword(ctx context.Context, email string) error {
	s.Logger.LogStart("AuthForgetPassword", "Reset password request for: %s", email)

	user, err := s.repo.User.FindByEmail(nil, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.Logger.LogEnd("AuthForgetPassword", "Unknown email, nothing sent: %s", email)
			return nil
		}
		s.Logger.LogEndWithError("AuthForgetPassword", "Failed to look up user: %v", err)
		return err
	}

	token, err := helpers.GenerateRandomString(32)
	if err != nil {
		s.Logger.LogEndWithError("AuthForgetPassword", "Reset password failed - token generation error: %v", err)
		return err
	}

	// SHA-256 (deterministic) so AuthResetPassword can look the token up with WHERE token = ?.
	passwordReset := &models.PasswordReset{
		Email:     user.Email,
		Token:     helpers.HashToken(token),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      false,
	}

	if err := s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
		// Only the newest link stays valid.
		if err := s.repo.PasswordReset.InvalidateByEmail(tx, user.Email); err != nil {
			return err
		}
		_, err := s.repo.PasswordReset.Create(tx, passwordReset)
		return err
	}); err != nil {
		s.Logger.LogEndWithError("AuthForgetPassword", "Reset password failed - database error: %v", err)
		return err
	}
	s.Logger.LogStep("AuthForgetPassword", "Reset token saved to database")

	_ = s.NotificationCreate(ctx, &NotificationCreateParams{
		UserID:  user.ID,
		Type:    "info",
		Title:   "Password Reset Requested",
		Message: fmt.Sprintf("Password reset requested for %s", user.Email),
		Data: map[string]interface{}{
			"email": user.Email,
		},
	})

	resetURL := helpers.GetEnv("APP_FE_URL", "http://localhost:3000") + "/reset-password?token=" + token

	s.runAsync("AuthForgetPassword.sendEmail", func() {
		if err := s.EmailClient.SendResetPasswordEmail(user.Email, token, resetURL); err != nil {
			s.Logger.LogError("AuthForgetPassword", "Failed to send reset email to %s: %v", user.Email, err)
			return
		}
		s.Logger.LogStep("AuthForgetPassword", "Reset email sent successfully to %s", user.Email)
	})

	s.Logger.LogEnd("AuthForgetPassword", "Reset password request processed successfully")
	return nil
}

// AuthResetPassword validates token and resets user password.
func (s *Services) AuthResetPassword(ctx context.Context, token, newPassword string) error {
	s.Logger.LogStart("AuthResetPassword", "Password reset attempt")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.Logger.LogEndWithError("AuthResetPassword", "Failed to hash password: %v", err)
		return err
	}

	var userID uint
	err = s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
		// FindByToken only returns unused tokens.
		reset, err := s.repo.PasswordReset.FindByToken(tx, helpers.HashToken(token))
		if err != nil {
			return helpers.ErrTokenInvalid
		}

		if reset.ExpiresAt.Before(time.Now()) {
			return helpers.ErrTokenExpired
		}

		user, err := s.repo.User.UpdateMap(tx, &models.User{Email: reset.Email}, map[string]interface{}{
			"password": string(hashedPassword),
		})
		if err != nil {
			return helpers.ErrNotFound
		}
		userID = user.ID

		// Burn this token and any other outstanding token for the same email.
		return s.repo.PasswordReset.InvalidateByEmail(tx, reset.Email)
	})
	if err != nil {
		s.Logger.LogEndWithError("AuthResetPassword", "Password reset failed: %v", err)
		return err
	}

	s.Access.Invalidate(userID)

	_ = s.NotificationCreate(ctx, &NotificationCreateParams{
		UserID:  userID,
		Type:    "success",
		Title:   "Password Reset Successful",
		Message: "Your password has been reset successfully",
		Data: map[string]interface{}{
			"id": userID,
		},
	})

	s.Logger.LogEnd("AuthResetPassword", "Password reset successful for user ID: %d", userID)
	return nil
}

// sendVerificationEmail generates a verification token, stores it in Redis, and emails it to the user.
func (s *Services) sendVerificationEmail(userID uint, email string) {
	if !s.RedisClient.IsCacheAvailable() {
		s.Logger.LogWarn("sendVerificationEmail", "Redis not available, skipping verification email")
		return
	}

	token, err := helpers.GenerateRandomString(32)
	if err != nil {
		s.Logger.LogError("sendVerificationEmail", "Failed to generate token: %v", err)
		return
	}

	key := fmt.Sprintf("verify_email:%s", token)
	userIDStr := strconv.FormatUint(uint64(userID), 10)
	if err := s.RedisClient.Set(key, userIDStr, 24*time.Hour); err != nil {
		s.Logger.LogError("sendVerificationEmail", "Failed to store token in Redis: %v", err)
		return
	}

	frontendURL := helpers.GetEnv("APP_FE_URL", "http://localhost:5173")
	verifyURL := frontendURL + "/verify-email?token=" + token

	if err := s.EmailClient.SendVerificationEmail(email, verifyURL); err != nil {
		s.Logger.LogError("sendVerificationEmail", "Failed to send verification email to %s: %v", email, err)
		s.Logger.LogWarn("sendVerificationEmail", "[DEV] Verify URL for %s: %s", email, verifyURL)
	} else {
		s.Logger.LogStep("sendVerificationEmail", "Verification email sent to %s", email)
	}
}

// AuthResendVerification sends (or resends) a verification email to the given address.
func (s *Services) AuthResendVerification(ctx context.Context, email string) error {
	s.Logger.LogStart("AuthResendVerification", "Resend verification request for: %s", email)

	user, err := s.repo.User.FindByEmail(nil, email)
	if err != nil {
		s.Logger.LogEndWithError("AuthResendVerification", "User not found: %s", email)
		return helpers.ErrNotFound
	}

	if user.EmailVerifiedAt != nil {
		s.Logger.LogEndWithError("AuthResendVerification", "Email already verified: %s", email)
		return &helpers.CustomError{Status: http.StatusBadRequest, Message: "email already verified"}
	}

	s.runAsync("AuthResendVerification.sendEmail", func() {
		s.sendVerificationEmail(user.ID, user.Email)
	})

	s.Logger.LogEnd("AuthResendVerification", "Verification email queued for: %s", email)
	return nil
}

// AuthVerifyEmail validates an email verification token and marks the email as verified.
func (s *Services) AuthVerifyEmail(ctx context.Context, token string) error {
	s.Logger.LogStart("AuthVerifyEmail", "Email verification attempt")

	if !s.RedisClient.IsCacheAvailable() {
		return helpers.ErrTokenInvalid
	}

	key := fmt.Sprintf("verify_email:%s", token)
	userIDStr, err := s.RedisClient.Get(key)
	if err != nil {
		s.Logger.LogEndWithError("AuthVerifyEmail", "Token not found or expired")
		return helpers.ErrTokenInvalid
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		s.Logger.LogEndWithError("AuthVerifyEmail", "Invalid userID stored in Redis")
		return helpers.ErrTokenInvalid
	}

	user, err := s.repo.User.FindByID(nil, uint(userID))
	if err != nil {
		return helpers.ErrNotFound
	}
	if user.EmailVerifiedAt != nil {
		return &helpers.CustomError{Status: http.StatusBadRequest, Message: "email already verified"}
	}

	now := time.Now()
	if err := s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
		_, err := s.repo.User.Update(tx, &models.User{ID: uint(userID)}, &models.User{EmailVerifiedAt: &now})
		return err
	}); err != nil {
		s.Logger.LogEndWithError("AuthVerifyEmail", "Failed to set email_verified_at: %v", err)
		return err
	}

	_ = s.RedisClient.Delete(key)

	s.Logger.LogEnd("AuthVerifyEmail", "Email verified for userID: %d", userID)
	return nil
}
