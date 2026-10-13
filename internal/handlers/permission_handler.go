package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

// PermissionGetAll handles GET /api/permissions with optional pagination
// (page_size omitted or -1 returns all records)
func (h *Handlers) PermissionGetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "-1"))

	if pageSize < 0 {
		page = 1
	}

	opts := &repositories.QueryOptions{
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svcs.PermissionGetAll(c.Request.Context(), opts)
	if helpers.HandleError(c, err, "Failed to fetch permissions") {
		return
	}

	helpers.OKWithMetadata(c, "Permissions fetched successfully", result)
}

// PermissionGetByID handles GET /api/permissions/:id
func (h *Handlers) PermissionGetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.BadRequest(c, "Invalid permission ID")
		return
	}

	dto, err := h.svcs.PermissionGetByID(c.Request.Context(), uint(id))
	if helpers.HandleError(c, err, "Failed to fetch permission") {
		return
	}

	helpers.OK(c, "Permission fetched successfully", dto)
}
