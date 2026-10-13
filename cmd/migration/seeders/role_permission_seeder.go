package seeders

import (
	"fmt"
	"log"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// SeedRolePermissions maps roles to their permissions.
func SeedRolePermissions(db *gorm.DB, roleIDs map[string]uint, permIDs map[string]uint) {
	fmt.Println("Seeding role permissions...")

	count := 0
	for _, role := range dtos.RbacRoles {
		roleID, roleOK := roleIDs[role.Name]
		if !roleOK {
			log.Printf("Role %s not found, skipping", role.Name)
			continue
		}

		desiredIDs, err := desiredPermissionIDs(role, permIDs)
		if err != nil {
			log.Printf("Failed to resolve permissions for role %s: %v", role.Name, err)
			continue
		}

		for permID := range desiredIDs {
			var existing models.RoleHasPermission
			err := db.Where("role_id = ? AND permission_id = ?", roleID, permID).First(&existing).Error
			if err == nil {
				continue
			}

			rp := models.RoleHasPermission{
				RoleID:       roleID,
				PermissionID: permID,
			}

			if err := db.Create(&rp).Error; err != nil {
				log.Printf("Failed to create role_permission for %s (permission %d): %v", role.Name, permID, err)
			} else {
				count++
			}
		}
	}

	fmt.Printf("✓ Seeded %d role permissions\n", count)
}
