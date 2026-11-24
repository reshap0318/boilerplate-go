package repositories

import (
	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/models"
)

// PasswordResetRepository provides database operations for PasswordReset model.
type PasswordResetRepository struct {
	*GenericRepository[models.PasswordReset]
}

// NewPasswordResetRepository creates a new PasswordResetRepository.
func NewPasswordResetRepository(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{
		GenericRepository: NewGenericRepository(db, &models.PasswordReset{}),
	}
}

// FindByEmail finds password reset records by email.
func (r *PasswordResetRepository) FindByEmail(email string) ([]*models.PasswordReset, error) {
	var resets []*models.PasswordReset
	query := r.DB.Where("email = ? AND used = ?", email, false).Find(&resets)
	return resets, query.Error
}

// FindByToken finds a password reset record by token.
func (r *PasswordResetRepository) FindByToken(tx *gorm.DB, token string) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	db := r.getDB(tx)
	query := db.Where("token = ? AND used = ?", token, false).First(&reset)
	if query.Error != nil {
		return nil, query.Error
	}
	return &reset, nil
}

// InvalidateByEmail marks all unused password reset tokens for an email as used.
func (r *PasswordResetRepository) InvalidateByEmail(tx *gorm.DB, email string) error {
	return r.getDB(tx).Model(&models.PasswordReset{}).
		Where("email = ? AND used = ?", email, false).
		Update("used", true).Error
}

// InvalidateToken marks a specific token as used.
func (r *PasswordResetRepository) InvalidateToken(token string) error {
	query := r.DB.Model(&models.PasswordReset{}).Where("token = ?", token).Update("used", true)
	return query.Error
}
