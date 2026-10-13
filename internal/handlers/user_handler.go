package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

// UserCreate handles POST /api/users
func (h *Handlers) UserCreate(c *gin.Context) {
	var req dtos.UserCreateRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	dto, err := h.svcs.UserCreate(c.Request.Context(), req)
	if helpers.HandleError(c, err, "Failed to create user") {
		return
	}

	helpers.Created(c, "User created successfully", dto)
}

// UserGetAll handles GET /api/users with optional pagination and role filter
// (page_size omitted or -1 returns all records)
func (h *Handlers) UserGetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "-1"))

	if pageSize < 0 {
		page = 1
	}

	var roleID uint
	if roleIDQuery := c.Query("role_id"); roleIDQuery != "" {
		parsed, err := strconv.ParseUint(roleIDQuery, 10, 64)
		if err != nil {
			helpers.BadRequest(c, "Invalid role ID")
			return
		}
		roleID = uint(parsed)
	}

	opts := &repositories.QueryOptions{
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svcs.UserGetAll(c.Request.Context(), opts, roleID)
	if helpers.HandleError(c, err, "Failed to fetch users") {
		return
	}

	helpers.OKWithMetadata(c, "Users fetched successfully", result)
}

// UserGetByID handles GET /api/users/:id
func (h *Handlers) UserGetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.BadRequest(c, "Invalid user ID")
		return
	}

	dto, err := h.svcs.UserGetByID(c.Request.Context(), uint(id))
	if helpers.HandleError(c, err, "Failed to fetch user") {
		return
	}

	helpers.OK(c, "User fetched successfully", dto)
}

// UserUpdate handles PUT /api/users/:id
func (h *Handlers) UserUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.BadRequest(c, "Invalid user ID")
		return
	}

	var req dtos.UserUpdateRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if req.Password == "" {
		req.PasswordConfirmation = ""
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	dto, err := h.svcs.UserUpdate(c.Request.Context(), uint(id), req)
	if helpers.HandleError(c, err, "Failed to update user") {
		return
	}

	helpers.OK(c, "User updated successfully", dto)
}

// UserDelete handles DELETE /api/users/:id
func (h *Handlers) UserDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.BadRequest(c, "Invalid user ID")
		return
	}

	err = h.svcs.UserDelete(c.Request.Context(), uint(id))
	if helpers.HandleError(c, err, "Failed to delete user") {
		return
	}

	helpers.OK(c, "User deleted successfully", nil)
}

// ProfileGet handles GET /api/me
func (h *Handlers) ProfileGet(c *gin.Context) {
	userID := c.GetUint("user_id")

	dto, err := h.svcs.ProfileGet(c.Request.Context(), userID)
	if helpers.HandleError(c, err, "Failed to fetch profile") {
		return
	}

	helpers.OK(c, "Profile fetched successfully", dto)
}

// ProfileUpdate handles PUT /api/me
func (h *Handlers) ProfileUpdate(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req dtos.ProfileUpdateRequest

	if err := c.BindJSON(&req); err != nil {
		helpers.BadRequest(c, "Invalid JSON payload")
		return
	}

	if req.Password == "" {
		req.PasswordConfirmation = ""
	}

	if err := h.Validate.Struct(req); err != nil {
		helpers.ValidationResponse(c, h.getErrorsMap(err))
		return
	}

	dto, err := h.svcs.ProfileUpdate(c.Request.Context(), userID, req)
	if helpers.HandleError(c, err, "Failed to update profile") {
		return
	}

	helpers.OK(c, "Profile updated successfully", dto)
}
