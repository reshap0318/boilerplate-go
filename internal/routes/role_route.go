package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/handlers"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/middleware"
)

// RegisterRoleRoutes registers protected role routes.
func RegisterRoleRoutes(r *gin.RouterGroup, h *handlers.Handlers, acc *helpers.Access) {
	roles := r.Group("/roles")
	{
		roles.POST("", middleware.RequirePermission(acc, dtos.PermissionRoleCreate), h.RoleCreate)
		roles.GET("", middleware.RequirePermission(acc, dtos.PermissionRoleIndex), h.RoleGetAll)
		roles.GET("/:id", middleware.RequirePermission(acc, dtos.PermissionRoleIndex), h.RoleGetByID)
		roles.PUT("/:id", middleware.RequirePermission(acc, dtos.PermissionRoleEdit), h.RoleUpdate)
		roles.DELETE("/:id", middleware.RequirePermission(acc, dtos.PermissionRoleDelete), h.RoleDelete)
		roles.GET("/:id/permissions", middleware.RequirePermission(acc, dtos.PermissionRoleIndex), h.RoleGetPermissions)
	}
}
