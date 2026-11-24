package repositories

import (
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// UserRoleRepository provides database operations for UserHasRole model.
type UserRoleRepository struct {
	*GenericRepository[models.UserHasRole]
}

// NewUserRoleRepository creates a new UserRoleRepository.
func NewUserRoleRepository(db *gorm.DB) *UserRoleRepository {
	return &UserRoleRepository{
		GenericRepository: NewGenericRepository(db, &models.UserHasRole{}),
	}
}

// DeleteByUserID deletes all user_has_roles for a given user ID.
func (r *UserRoleRepository) DeleteByUserID(tx *gorm.DB, userID uint) error {
	return tx.Where("user_id = ?", userID).Delete(&models.UserHasRole{}).Error
}

// FindUserIDsByRoleID finds all user IDs holding a given role.
func (r *UserRoleRepository) FindUserIDsByRoleID(roleID uint) ([]uint, error) {
	var userIDs []uint
	err := r.DB.Table("user_has_roles").
		Where("role_id = ?", roleID).
		Pluck("user_id", &userIDs).Error
	return userIDs, err
}

// FindAccessNames returns the role names and permission names of an active user
// in ONE query (users → user_has_roles → roles → role_has_permissions → permissions).
// It replaces Preload("Roles.Permissions"), which costs 5 queries and loads full
// rows when only names are needed. Soft-deleted users/roles/permissions are excluded.
// Signature matches helpers.AccessLoader so the DI container can inject it.
func (r *UserRoleRepository) FindAccessNames(userID uint) (roles []string, permissions []string, err error) {
	type row struct {
		RoleName       string
		PermissionName *string
	}
	var rows []row
	err = r.DB.Table("user_has_roles AS uhr").
		Select("roles.name AS role_name, permissions.name AS permission_name").
		Joins("INNER JOIN users ON users.id = uhr.user_id AND users.deleted_at IS NULL").
		Joins("INNER JOIN roles ON roles.id = uhr.role_id AND roles.deleted_at IS NULL").
		Joins("LEFT JOIN role_has_permissions rhp ON rhp.role_id = roles.id").
		Joins("LEFT JOIN permissions ON permissions.id = rhp.permission_id AND permissions.deleted_at IS NULL").
		Where("uhr.user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, nil, err
	}

	roleSet := make(map[string]struct{})
	permSet := make(map[string]struct{})
	for _, rw := range rows {
		if _, ok := roleSet[rw.RoleName]; !ok {
			roleSet[rw.RoleName] = struct{}{}
			roles = append(roles, rw.RoleName)
		}
		if rw.PermissionName != nil {
			if _, ok := permSet[*rw.PermissionName]; !ok {
				permSet[*rw.PermissionName] = struct{}{}
				permissions = append(permissions, *rw.PermissionName)
			}
		}
	}
	return roles, permissions, nil
}
