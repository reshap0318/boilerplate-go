package seeders

import (
	"errors"
	"fmt"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// SyncRBAC syncs the permissions and roles defined in dtos with the database.
func SyncRBAC(db *gorm.DB) error {
	fmt.Println("Syncing RBAC...")

	var permIDs map[string]uint
	var roleIDs map[string]uint

	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error

		permIDs, err = syncPermissions(tx)
		if err != nil {
			return err
		}

		roleIDs, err = syncRoles(tx)
		if err != nil {
			return err
		}

		return syncRolePermissions(tx, roleIDs, permIDs)
	}); err != nil {
		return err
	}

	fmt.Printf("✓ RBAC synced (%d permissions, %d roles)\n", len(permIDs), len(roleIDs))
	return nil
}

// syncPermissions upserts dtos.RbacPermissions.
func syncPermissions(db *gorm.DB) (map[string]uint, error) {
	resultMap := make(map[string]uint)
	definedNames := make([]string, 0, len(dtos.RbacPermissions))

	for _, perm := range dtos.RbacPermissions {
		definedNames = append(definedNames, perm.Name)

		var existing models.Permission
		err := db.Unscoped().Where("name = ?", perm.Name).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			permission := models.Permission{
				Name:        perm.Name,
				Description: strPtr(perm.Description),
			}
			if err := db.Create(&permission).Error; err != nil {
				return nil, fmt.Errorf("create permission %s: %w", perm.Name, err)
			}
			resultMap[perm.Name] = permission.ID
			fmt.Printf("  + permission %s\n", perm.Name)

		case err != nil:
			return nil, fmt.Errorf("find permission %s: %w", perm.Name, err)

		default:
			updates := map[string]interface{}{}
			if existing.DeletedAt.Valid {
				updates["deleted_at"] = nil
			}
			if existing.Description == nil || *existing.Description != perm.Description {
				updates["description"] = perm.Description
			}
			if len(updates) > 0 {
				if err := db.Unscoped().Model(&existing).Updates(updates).Error; err != nil {
					return nil, fmt.Errorf("update permission %s: %w", perm.Name, err)
				}
				fmt.Printf("  ~ permission %s\n", perm.Name)
			}
			resultMap[perm.Name] = existing.ID
		}
	}

	stalePermissions, err := findStalePermissions(db, definedNames)
	if err != nil {
		return nil, err
	}
	if len(stalePermissions) > 0 {
		staleIDs := make([]uint, 0, len(stalePermissions))
		for _, perm := range stalePermissions {
			staleIDs = append(staleIDs, perm.ID)
			fmt.Printf("  - permission %s (removed from code)\n", perm.Name)
		}

		if err := db.Where("permission_id IN ?", staleIDs).Delete(&models.RoleHasPermission{}).Error; err != nil {
			return nil, fmt.Errorf("delete role mappings of stale permissions: %w", err)
		}
		if err := db.Where("id IN ?", staleIDs).Delete(&models.Permission{}).Error; err != nil {
			return nil, fmt.Errorf("delete stale permissions: %w", err)
		}
	}

	return resultMap, nil
}

// findStalePermissions returns permissions no longer defined in code.
func findStalePermissions(db *gorm.DB, definedNames []string) ([]models.Permission, error) {
	if len(definedNames) == 0 {
		return nil, nil
	}

	var stale []models.Permission
	if err := db.Where("name NOT IN ?", definedNames).Find(&stale).Error; err != nil {
		return nil, fmt.Errorf("find stale permissions: %w", err)
	}
	return stale, nil
}

// syncRoles upserts the roles defined in dtos.RbacRoles.
func syncRoles(db *gorm.DB) (map[string]uint, error) {
	resultMap := make(map[string]uint)

	for _, role := range dtos.RbacRoles {
		var existing models.Role
		err := db.Unscoped().Where("name = ?", role.Name).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			newRole := models.Role{
				Name:        role.Name,
				Description: strPtr(role.Description),
			}
			if err := db.Create(&newRole).Error; err != nil {
				return nil, fmt.Errorf("create role %s: %w", role.Name, err)
			}
			resultMap[role.Name] = newRole.ID
			fmt.Printf("  + role %s\n", role.Name)

		case err != nil:
			return nil, fmt.Errorf("find role %s: %w", role.Name, err)

		default:
			updates := map[string]interface{}{}
			if existing.DeletedAt.Valid {
				updates["deleted_at"] = nil
			}
			if existing.Description == nil || *existing.Description != role.Description {
				updates["description"] = role.Description
			}
			if len(updates) > 0 {
				if err := db.Unscoped().Model(&existing).Updates(updates).Error; err != nil {
					return nil, fmt.Errorf("update role %s: %w", role.Name, err)
				}
				fmt.Printf("  ~ role %s\n", role.Name)
			}
			resultMap[role.Name] = existing.ID
		}
	}

	return resultMap, nil
}

// syncRolePermissions syncs the permission set of every system role.
func syncRolePermissions(db *gorm.DB, roleIDs, permIDs map[string]uint) error {
	for _, role := range dtos.RbacRoles {
		roleID, ok := roleIDs[role.Name]
		if !ok {
			return fmt.Errorf("role %s not found after sync", role.Name)
		}

		desiredIDs, err := desiredPermissionIDs(role, permIDs)
		if err != nil {
			return err
		}

		var current []models.RoleHasPermission
		if err := db.Where("role_id = ?", roleID).Find(&current).Error; err != nil {
			return fmt.Errorf("load permissions of role %s: %w", role.Name, err)
		}

		currentIDs := make(map[uint]struct{}, len(current))
		for _, rp := range current {
			currentIDs[rp.PermissionID] = struct{}{}
			if _, keep := desiredIDs[rp.PermissionID]; keep {
				continue
			}
			if err := db.Where("role_id = ? AND permission_id = ?", roleID, rp.PermissionID).
				Delete(&models.RoleHasPermission{}).Error; err != nil {
				return fmt.Errorf("remove permission %d from role %s: %w", rp.PermissionID, role.Name, err)
			}
		}

		for permID := range desiredIDs {
			if _, exists := currentIDs[permID]; exists {
				continue
			}
			rp := models.RoleHasPermission{
				RoleID:       roleID,
				PermissionID: permID,
			}
			if err := db.Create(&rp).Error; err != nil {
				return fmt.Errorf("assign permission %d to role %s: %w", permID, role.Name, err)
			}
		}
	}

	return nil
}

// desiredPermissionIDs returns the permission IDs a role must hold.
func desiredPermissionIDs(role dtos.RbacRole, permIDs map[string]uint) (map[uint]struct{}, error) {
	names := role.Permissions
	if role.Name == dtos.RoleSuperAdmin {
		names = make([]string, 0, len(permIDs))
		for name := range permIDs {
			names = append(names, name)
		}
	}

	desired := make(map[uint]struct{}, len(names))
	for _, name := range names {
		permID, ok := permIDs[name]
		if !ok {
			return nil, fmt.Errorf("permission %s referenced by role %s is not defined", name, role.Name)
		}
		desired[permID] = struct{}{}
	}

	return desired, nil
}
