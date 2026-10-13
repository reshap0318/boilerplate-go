package seeders

import (
	"fmt"
	"log"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// SeedPermissions inserts default permission data.
func SeedPermissions(db *gorm.DB) map[string]uint {
	fmt.Println("Seeding permissions...")

	resultMap := make(map[string]uint)

	for _, perm := range dtos.RbacPermissions {
		var existing models.Permission
		err := db.Where("name = ?", perm.Name).First(&existing).Error
		if err == nil {
			resultMap[perm.Name] = existing.ID
			fmt.Printf("  ⊘ Permission %s already exists, skipping\n", perm.Name)
			continue
		}

		p := models.Permission{
			Name:        perm.Name,
			Description: strPtr(perm.Description),
		}

		if err := db.Create(&p).Error; err != nil {
			log.Printf("Failed to create permission %s: %v", perm.Name, err)
		} else {
			resultMap[perm.Name] = p.ID
		}
	}

	fmt.Printf("✓ Seeded %d permissions\n", len(resultMap))
	return resultMap
}

func strPtr(s string) *string {
	return &s
}
