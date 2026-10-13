package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

// RoleCreate creates a new role with permissions.
func (s *Services) RoleCreate(ctx context.Context, req dtos.RoleRequest) (*dtos.RoleDTO, error) {
	s.Logger.LogStart("RoleCreate", "Creating role: %s", req.Name)

	role := &models.Role{
		Name:        req.Name,
		Description: req.Description,
	}

	var result *models.Role
	res, err := s.repo.TxManager.WithinTransactionWithResult(func(tx *gorm.DB) (interface{}, error) {
		var err error
		result, err = s.repo.Role.Create(tx, role)
		if err != nil {
			return nil, err
		}

		// Assign permissions
		var perms []models.Permission
		for _, permID := range req.Permissions {
			perms = append(perms, models.Permission{ID: permID})
		}
		if err := tx.Model(&result).Association("Permissions").Append(perms); err != nil {
			s.Logger.LogStep("RoleCreate", "Failed to assign permissions: %v", err)
		}

		// Reload role with permissions
		reloaded, err := s.repo.Role.FindByID(tx, result.ID, "Permissions")
		if err != nil {
			return nil, err
		}

		return reloaded, nil
	})
	if err != nil {
		s.Logger.LogEndWithError("RoleCreate", "Failed to create role: %v", err)
		return nil, err
	}

	result = res.(*models.Role)

	dto := dtos.ToRoleDTO(result)

	_ = s.NotificationCreate(ctx, &NotificationCreateParams{
		Type:    "success",
		Title:   "Role Created",
		Message: fmt.Sprintf("New role created: %s", req.Name),
		Data: map[string]interface{}{
			"id":   result.ID,
			"name": result.Name,
		},
	})

	s.Logger.LogEnd("RoleCreate", "Role created: %s (ID: %d)", dto.Name, dto.ID)
	return &dto, nil
}

// RoleGetAll returns roles with permissions, paginated when opts.PageSize > 0, otherwise all records.
func (s *Services) RoleGetAll(ctx context.Context, opts *repositories.QueryOptions) (*repositories.PagedResult[dtos.RoleDTO], error) {
	if opts == nil {
		opts = &repositories.QueryOptions{}
	}
	if opts.SortBy == "" {
		opts.SortBy = "id"
	}
	if opts.Order == "" {
		opts.Order = "ASC"
	}
	opts.Preloads = []string{"Permissions"}

	if !s.Access.HasPermission(ctx, dtos.PermissionRoleIndexSu) {
		opts.ConditionGroups = append(opts.ConditionGroups, repositories.ConditionGroup{
			Logic: "AND",
			Conditions: []repositories.QueryCondition{
				{Column: "id", Operator: "!=", Value: 1},
			},
		})
	}

	result, err := s.repo.Role.FindAllWithOpts(nil, opts)
	if err != nil {
		return nil, err
	}

	dtoList := dtos.ToRoleDTOList(result.Data)

	return &repositories.PagedResult[dtos.RoleDTO]{
		Data:       dtoList,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}, nil
}

// RoleGetByID returns a role by ID with permissions.
func (s *Services) RoleGetByID(ctx context.Context, id uint) (*dtos.RoleDTO, error) {
	if id == 1 && !s.Access.HasPermission(ctx, dtos.PermissionRoleIndexSu) {
		return nil, helpers.ErrForbidden
	}

	role, err := s.repo.Role.FindByID(nil, id, "Permissions")
	if err != nil {
		return nil, helpers.ErrNotFound
	}

	dto := dtos.ToRoleDTO(role)
	return &dto, nil
}

// RoleUpdate updates an existing role with permissions.
func (s *Services) RoleUpdate(ctx context.Context, id uint, req dtos.RoleRequest) (*dtos.RoleDTO, error) {
	s.Logger.LogStart("RoleUpdate", "Updating role ID: %d", id)

	if id == 1 && !s.Access.HasPermission(ctx, dtos.PermissionRoleIndexSu) {
		s.Logger.LogEndWithError("RoleUpdate", "Forbidden access to role ID: %d", id)
		return nil, helpers.ErrForbidden
	}

	role := &models.Role{ID: id}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != nil {
		role.Description = req.Description
	}

	var result *models.Role
	res, err := s.repo.TxManager.WithinTransactionWithResult(func(tx *gorm.DB) (interface{}, error) {
		var err error
		result, err = s.repo.Role.Update(tx, &models.Role{ID: id}, role)
		if err != nil {
			return nil, err
		}

		// Replace permissions - clear then assign
		if err := tx.Model(&result).Association("Permissions").Clear(); err != nil {
			return nil, err
		}

		var perms []models.Permission
		for _, permID := range req.Permissions {
			perms = append(perms, models.Permission{ID: permID})
		}
		if err := tx.Model(&result).Association("Permissions").Append(perms); err != nil {
			s.Logger.LogStep("RoleUpdate", "Failed to assign permissions: %v", err)
		}

		// Reload role with permissions
		reloaded, err := s.repo.Role.FindByID(tx, result.ID, "Permissions")
		if err != nil {
			return nil, err
		}

		return reloaded, nil
	})
	if err != nil {
		s.Logger.LogEndWithError("RoleUpdate", "Failed to update role: %v", err)
		return nil, err
	}

	result = res.(*models.Role)

	dto := dtos.ToRoleDTO(result)

	_ = s.NotificationCreate(ctx, &NotificationCreateParams{
		Type:    "info",
		Title:   "Role Updated",
		Message: fmt.Sprintf("Role updated: %s", result.Name),
		Data: map[string]interface{}{
			"id":   result.ID,
			"name": result.Name,
		},
	})

	go s.invalidateAccessForRole(id)

	s.Logger.LogEnd("RoleUpdate", "Role updated: %s (ID: %d)", dto.Name, dto.ID)
	return &dto, nil
}

// RoleDelete soft deletes a role.
func (s *Services) RoleDelete(ctx context.Context, id uint) error {
	s.Logger.LogStart("RoleDelete", "Deleting role ID: %d", id)

	if id == 1 && !s.Access.HasPermission(ctx, dtos.PermissionRoleIndexSu) {
		s.Logger.LogEndWithError("RoleDelete", "Forbidden access to role ID: %d", id)
		return helpers.ErrForbidden
	}

	if err := s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
		role := models.Role{ID: id}
		if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
			return err
		}
		_, err := s.repo.Role.Delete(tx, id)
		return err
	}); err != nil {
		s.Logger.LogEndWithError("RoleDelete", "Failed to delete role: %v", err)
		return err
	}

	_ = s.NotificationCreate(ctx, &NotificationCreateParams{
		Type:    "warning",
		Title:   "Role Deleted",
		Message: fmt.Sprintf("Role deleted: ID %d", id),
		Data: map[string]interface{}{
			"id": id,
		},
	})

	go s.invalidateAccessForRole(id)

	s.Logger.LogEnd("RoleDelete", "Role deleted: ID: %d", id)
	return nil
}

// invalidateAccessForRole clears the cached permission/role data for every
// user holding roleID, so a role's permission change takes effect immediately
// instead of waiting for their session cache to expire.
func (s *Services) invalidateAccessForRole(roleID uint) {
	userIDs, err := s.repo.UserRole.FindUserIDsByRoleID(roleID)
	if err != nil {
		s.Logger.LogWarn("invalidateAccessForRole", "Failed to look up users for role %d: %v", roleID, err)
		return
	}
	for _, userID := range userIDs {
		s.Access.Invalidate(userID)
	}
}

// RoleGetPermissions returns all permissions for a role.
func (s *Services) RoleGetPermissions(ctx context.Context, roleID uint) ([]dtos.PermissionDTO, error) {
	role, err := s.repo.Role.FindByID(nil, roleID, "Permissions")
	if err != nil {
		return nil, helpers.ErrNotFound
	}

	result := make([]dtos.PermissionDTO, len(role.Permissions))
	for i, p := range role.Permissions {
		result[i] = dtos.ToPermissionDTO(&p)
	}

	return result, nil
}
