package repositories

import (
	"time"

	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	*GenericRepository[models.Notification]
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{
		GenericRepository: NewGenericRepository(db, &models.Notification{}),
	}
}

func (r *NotificationRepository) FindByUserIDWithOpts(userID uint, opts *QueryOptions) (*PagedResult[models.Notification], error) {
	db := r.getDB(nil)
	query := db.Where("user_id = ?", userID)

	if opts != nil {
		if opts.Page <= 0 {
			opts.Page = 1
		}
		if opts.PageSize <= 0 {
			opts.PageSize = 10
		}
		if opts.SortBy == "" {
			opts.SortBy = "created_at"
		}
		if opts.Order == "" {
			opts.Order = "DESC"
		}
	}

	return r.findPaged(query.Model(&models.Notification{}), opts)
}

func (r *NotificationRepository) FindByUserIDAndFilters(userID uint, opts *QueryOptions, isRead *bool, notifType string) (*PagedResult[models.Notification], error) {
	db := r.getDB(nil)
	query := db.Where("user_id = ?", userID)

	if isRead != nil {
		if *isRead {
			query = query.Where("read_at IS NOT NULL")
		} else {
			query = query.Where("read_at IS NULL")
		}
	}

	if notifType != "" {
		query = query.Where("type = ?", notifType)
	}

	// Notifications are unbounded per user — a negative page size falls back to
	// the default instead of meaning "return everything".
	if opts != nil && opts.PageSize < 0 {
		opts.PageSize = 10
	}
	return r.findPaged(query.Model(&models.Notification{}), opts)
}

func (r *NotificationRepository) MarkAsRead(notificationID, userID uint) error {
	now := time.Now()
	return r.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("read_at", now).Error
}

func (r *NotificationRepository) MarkAllAsRead(userID uint) error {
	now := time.Now()
	return r.DB.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

func (r *NotificationRepository) CountUnread(userID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&count).Error
	return count, err
}
