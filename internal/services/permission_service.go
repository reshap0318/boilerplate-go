package services

import (
	"context"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

// PermissionGetAll returns permissions, paginated when opts.PageSize > 0, otherwise all records.
func (s *Services) PermissionGetAll(ctx context.Context, opts *repositories.QueryOptions) (*repositories.PagedResult[dtos.PermissionDTO], error) {
	if opts == nil {
		opts = &repositories.QueryOptions{}
	}
	if opts.SortBy == "" {
		opts.SortBy = "id"
	}
	if opts.Order == "" {
		opts.Order = "ASC"
	}

	result, err := s.repo.Permission.FindAllWithOpts(nil, opts)
	if err != nil {
		return nil, err
	}

	dtoList := dtos.ToPermissionDTOList(result.Data)

	return &repositories.PagedResult[dtos.PermissionDTO]{
		Data:       dtoList,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}, nil
}

// PermissionGetByID returns a permission by ID.
func (s *Services) PermissionGetByID(ctx context.Context, id uint) (*dtos.PermissionDTO, error) {
	permission, err := s.repo.Permission.FindByID(nil, id)
	if err != nil {
		return nil, helpers.ErrNotFound
	}

	dto := dtos.ToPermissionDTO(permission)
	return &dto, nil
}
