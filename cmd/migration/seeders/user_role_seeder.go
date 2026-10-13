package seeders

import (
	"fmt"
	"log"

	"github.com/reshap0318/go-boilerplate/internal/dtos"
	"github.com/reshap0318/go-boilerplate/internal/models"
	"gorm.io/gorm"
)

// SeedUserRoles maps users to roles.
func SeedUserRoles(db *gorm.DB, userEmails map[string]uint, roleIDs map[string]uint) {
	fmt.Println("Seeding user roles...")

	userRoles := map[string]string{
		"suAdmin@app.com": dtos.RoleSuperAdmin,
		"admin@app.com":   dtos.RoleAdmin,
		"editor@app.com":  dtos.RoleEditor,
		"viewer@app.com":  dtos.RoleViewer,
	}

	count := 0
	for email, roleName := range userRoles {
		userID, userOK := userEmails[email]
		if !userOK {
			log.Printf("User %s not found, skipping", email)
			continue
		}

		roleID, roleOK := roleIDs[roleName]
		if !roleOK {
			log.Printf("Role %s not found, skipping", roleName)
			continue
		}

		var existing models.UserHasRole
		err := db.Where("user_id = ? AND role_id = ?", userID, roleID).First(&existing).Error
		if err == nil {
			fmt.Printf("  ⊘ User role %s-%s already exists, skipping\n", email, roleName)
			continue
		}

		ur := models.UserHasRole{
			UserID: userID,
			RoleID: roleID,
		}

		if err := db.Create(&ur).Error; err != nil {
			log.Printf("Failed to create user_role for %s-%s: %v", email, roleName, err)
		} else {
			count++
		}
	}

	fmt.Printf("✓ Seeded %d user roles\n", count)
}
