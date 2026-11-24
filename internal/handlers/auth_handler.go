package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
)

// AuthLogin handles user login.
// @Summary User login
// @Description Authenticate user and return JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.LoginRequest true "Login credentials"
// @Success 200 {object} dtos.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/auth/login [post]
func (h *Handlers) AuthLogin(c *gin.Context) {
	var req dtos.LoginRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	response, err := h.svcs.AuthLogin(c.Request.Context(), req.Email, req.Password)
	if helpers.HandleError(c, err, "Login failed") {
		return
	}

	helpers.OK(c, "Login successful", response)
}

// AuthRefreshToken handles token refresh.
// @Summary Refresh access token
// @Description Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} dtos.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/auth/refresh [post]
func (h *Handlers) AuthRefreshToken(c *gin.Context) {
	var req dtos.RefreshTokenRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	response, err := h.svcs.AuthRefreshToken(c.Request.Context(), req.RefreshToken)
	if helpers.HandleError(c, err, "Token refresh failed") {
		return
	}

	helpers.OK(c, "Token refreshed successfully", response)
}

// AuthLogout handles user logout.
// @Summary User logout
// @Description Blacklists the current access token (and the refresh token, if sent in the body) so they cannot be reused
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.LogoutRequest false "Optional refresh token to revoke"
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /api/auth/logout [post]
func (h *Handlers) AuthLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		helpers.Unauthorized(c, "Invalid authorization header format")
		return
	}
	tokenString := parts[1]

	// Optional body: {"refresh_token": "..."} — revokes the refresh token too.
	var req dtos.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.svcs.AuthLogout(c.Request.Context(), tokenString, req.RefreshToken); err != nil {
		if helpers.HandleError(c, err, "Logout failed") {
			return
		}
	}

	helpers.OK(c, "Logout successful", nil)
}

// AuthForgetPassword handles forget password request.
// @Summary Forget password
// @Description Send reset password email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.ForgetPasswordRequest true "Email"
// @Success 200 {object} dtos.MessageResponse
// @Failure 400 {object} map[string]string
// @Router /api/auth/forgot-password [post]
func (h *Handlers) AuthForgetPassword(c *gin.Context) {
	var req dtos.ForgetPasswordRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	if err := h.svcs.AuthForgetPassword(c.Request.Context(), req.Email); err != nil {
		if helpers.HandleError(c, err, err.Error()) {
			return
		}
	}

	helpers.OK(c, "Reset password email sent", nil)
}

// AuthResetPassword handles reset password request.
// @Summary Reset password
// @Description Reset password with token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.ResetPasswordRequest true "Token and new password"
// @Success 200 {object} dtos.MessageResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/auth/reset-password [post]
func (h *Handlers) AuthResetPassword(c *gin.Context) {
	var req dtos.ResetPasswordRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	if err := h.svcs.AuthResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		if helpers.HandleError(c, err, err.Error()) {
			return
		}
	}

	helpers.OK(c, "Password has been reset successfully", nil)
}

// AuthResendVerification handles resend verification email request.
// @Summary Resend verification email
// @Description Resend the email verification link
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.ResendVerificationRequest true "Email"
// @Success 200 {object} dtos.MessageResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/auth/resend-verification [post]
func (h *Handlers) AuthResendVerification(c *gin.Context) {
	var req dtos.ResendVerificationRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	if err := h.svcs.AuthResendVerification(c.Request.Context(), req.Email); err != nil {
		if helpers.HandleError(c, err, err.Error()) {
			return
		}
	}

	helpers.OK(c, "Verification email sent", nil)
}

// AuthVerifyEmail handles email verification request.
// @Summary Verify email
// @Description Verify user email using the token sent via email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dtos.VerifyEmailRequest true "Verification token"
// @Success 200 {object} dtos.MessageResponse
// @Failure 400 {object} map[string]string
// @Router /api/auth/verify-email [post]
func (h *Handlers) AuthVerifyEmail(c *gin.Context) {
	var req dtos.VerifyEmailRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	if err := h.svcs.AuthVerifyEmail(c.Request.Context(), req.Token); err != nil {
		if helpers.HandleError(c, err, err.Error()) {
			return
		}
	}

	helpers.OK(c, "Email verified successfully", nil)
}
