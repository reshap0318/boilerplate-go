package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/handlers"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/middleware"
)

// RegisterPermissionRoutes registers protected permission routes.
func RegisterPermissionRoutes(r *gin.RouterGroup, h *handlers.Handlers, acc *helpers.Access) {
	permissions := r.Group("/permissions")
	{
		permissions.GET("", middleware.RequirePermission(acc, dtos.PermissionIndex), h.PermissionGetAll)
		permissions.GET("/:id", middleware.RequirePermission(acc, dtos.PermissionIndex), h.PermissionGetByID)
	}
}
