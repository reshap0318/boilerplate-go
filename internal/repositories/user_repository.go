package repositories

import (
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// UserRepository provides database operations for User model.
type UserRepository struct {
	*GenericRepository[models.User]
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		GenericRepository: NewGenericRepository(db, &models.User{}),
	}
}

// FindByEmail finds a user by email (LIMIT 1), optionally preloading relations.
func (r *UserRepository) FindByEmail(tx *gorm.DB, email string, preloads ...string) (*models.User, error) {
	var user models.User
	query := applyPreloads(r.getDB(tx).Where("email = ?", email), preloads)
	if err := query.First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindAllByRoleWithOpts paginates users that hold roleID. The role filter is a
// subquery (users.id IN (SELECT user_id FROM user_has_roles WHERE role_id = ?)),
// so it stays a single round-trip no matter how many users hold the role.
func (r *UserRepository) FindAllByRoleWithOpts(tx *gorm.DB, roleID uint, opts *QueryOptions) (*PagedResult[models.User], error) {
	sub := r.getDB(tx).Table("user_has_roles").Select("user_id").Where("role_id = ?", roleID)
	base := r.getDB(tx).Model(&models.User{}).Where("users.id IN (?)", sub)
	return r.findPaged(base, opts)
}
