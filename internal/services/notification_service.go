package services

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

type NotificationCreateParams struct {
	// UserID is the recipient. When 0, the caller from ctx is used. Public
	// endpoints (no caller in ctx) must set it explicitly.
	UserID  uint
	Type    string
	Title   string
	Message string
	Data    map[string]interface{}
}

func (s *Services) NotificationCreate(ctx context.Context, params *NotificationCreateParams) error {
	userID := params.UserID
	if userID == 0 {
		userID = helpers.GetCallerID(ctx)
	}
	if userID == 0 {
		// No recipient (public endpoint without explicit UserID): inserting would
		// violate the notifications.user_id foreign key, so skip it.
		return nil
	}

	var dataJSON string
	if params.Data != nil {
		dataBytes, err := json.Marshal(params.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal notification data: %w", err)
		}
		dataJSON = string(dataBytes)
	}

	notification := &models.Notification{
		UserID:  userID,
		Type:    params.Type,
		Title:   params.Title,
		Message: params.Message,
		Data:    dataJSON,
	}

	_, err := s.repo.Notification.Create(nil, notification)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

func (s *Services) NotificationGetAllWithFilters(ctx context.Context, opts *repositories.QueryOptions, isRead *bool, notifType string) (*repositories.PagedResult[dtos.NotificationDTO], error) {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return nil, helpers.ErrInvalidToken
	}

	result, err := s.repo.Notification.FindByUserIDAndFilters(userID, opts, isRead, notifType)
	if err != nil {
		return nil, err
	}

	dtoList := dtos.ToNotificationDTOList(result.Data)

	return &repositories.PagedResult[dtos.NotificationDTO]{
		Data:       dtoList,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}, nil
}

func (s *Services) NotificationGetAll(ctx context.Context, opts *repositories.QueryOptions) (*repositories.PagedResult[dtos.NotificationDTO], error) {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return nil, helpers.ErrInvalidToken
	}

	result, err := s.repo.Notification.FindByUserIDWithOpts(userID, opts)
	if err != nil {
		return nil, err
	}

	dtoList := dtos.ToNotificationDTOList(result.Data)

	return &repositories.PagedResult[dtos.NotificationDTO]{
		Data:       dtoList,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}, nil
}

func (s *Services) NotificationGetByID(ctx context.Context, id uint) (*dtos.NotificationDTO, error) {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return nil, helpers.ErrInvalidToken
	}

	notification, err := s.repo.Notification.FindByID(nil, id)
	if err != nil {
		return nil, err
	}

	if notification.UserID != userID {
		return nil, helpers.ErrForbidden
	}

	dto := dtos.ToNotificationDTO(notification)
	return &dto, nil
}

func (s *Services) NotificationMarkAsRead(ctx context.Context, id uint) error {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return helpers.ErrInvalidToken
	}

	return s.repo.Notification.MarkAsRead(id, userID)
}

func (s *Services) NotificationMarkAllAsRead(ctx context.Context) error {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return helpers.ErrInvalidToken
	}

	return s.repo.Notification.MarkAllAsRead(userID)
}

func (s *Services) NotificationCountUnread(ctx context.Context) (int64, error) {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return 0, helpers.ErrInvalidToken
	}

	return s.repo.Notification.CountUnread(userID)
}

func (s *Services) NotificationDelete(ctx context.Context, id uint) error {
	userID := helpers.GetCallerID(ctx)
	if userID == 0 {
		return helpers.ErrInvalidToken
	}

	notification, err := s.repo.Notification.FindByID(nil, id)
	if err != nil {
		return err
	}

	if notification.UserID != userID {
		return helpers.ErrForbidden
	}

	return s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
		_, err = s.repo.Notification.Delete(tx, id)
		return err
	})
}
