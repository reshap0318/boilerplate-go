package dtos

import "github.com/reshap0318/go-boilerplate/internal/models"

// PermissionDTO represents permission data transfer object.
type PermissionDTO struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// ToPermissionDTO converts Permission model to PermissionDTO.
func ToPermissionDTO(p *models.Permission) PermissionDTO {
	return PermissionDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}
}

// ToPermissionDTOList converts a slice of Permission models to PermissionDTOs.
func ToPermissionDTOList(permissions []models.Permission) []PermissionDTO {
	result := make([]PermissionDTO, len(permissions))
	for i, p := range permissions {
		result[i] = ToPermissionDTO(&p)
	}
	return result
}
