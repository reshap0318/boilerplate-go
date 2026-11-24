package references

import (
	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/models"
)

// ============================================================
// Repository registration — add to 00_repository.go
// ============================================================
type Repositories struct {
	TxManager  *TransactionManager
	Permission *PermissionRepository
	// ... other repos
}

func NewRepositories(db *gorm.DB) (*Repositories, error) {
	return &Repositories{
		TxManager:  NewTransactionManager(db),
		Permission: NewPermissionRepository(db),
	}, nil
}

// ============================================================
// Repository — extends GenericRepository
// ============================================================
type PermissionRepository struct {
	*GenericRepository[models.Permission]
}

func NewPermissionRepository(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{
		GenericRepository: NewGenericRepository(db, &models.Permission{}),
	}
}

// ============================================================
// DECISION: generic method or custom repository method?
// ============================================================
//
// Ask in this order — stop at the first YES:
//
//  1. Single table, filter by equality on columns?            → FindByFieldMap / ExistsWithMap
//  2. List page: sort + WHERE (=, LIKE, IN, <, >) + relations? → FindAllWithOpts (QueryOptions)
//  3. One row by id (+ relations)?                             → FindByID / FindByIDWithOpts
//  4. Anything else → CUSTOM METHOD in the repository:
//     - JOIN to filter/sort by another table's column
//     - subquery (WHERE id IN (SELECT ...)), EXISTS
//     - aggregate (COUNT/SUM ... GROUP BY), DISTINCT
//     - selecting only some columns / scanning into a non-model struct
//     - bulk UPDATE/DELETE with a WHERE that is not "id = ?"
//     - IS NULL / IS NOT NULL conditions (QueryCondition always binds "?")
//     - a paginated list that needs any of the above → build the base query, then r.findPaged(base, opts)
//
// Whatever the answer, the query lives in the REPOSITORY. A service never calls
// r.DB / tx.Where / tx.Joins / tx.Raw itself; it only calls repository methods.
// (Exception: tx.Model(x).Association(...) for many-to-many writes inside a transaction.)

// ---------- 1. Generic is enough — do NOT write a custom method ----------
//
//     perms, err := s.repo.Permission.FindByFieldMap(nil, map[string]interface{}{"name": name})
//
// NOT RECOMMENDED (duplicates FindByFieldMap):
//     func (r *PermissionRepository) FindByName(name string) ([]models.Permission, error) {
//         var p []models.Permission
//         return p, r.DB.Where("name = ?", name).Find(&p).Error
//     }

// ---------- 2. Custom: JOIN filter (generic cannot reach another table) ----------
func (r *PermissionRepository) FindByRoleID(tx *gorm.DB, roleID uint) ([]models.Permission, error) {
	var permissions []models.Permission
	err := r.getDB(tx).Model(&models.Permission{}). // Model() keeps the soft-delete scope
		Joins("JOIN role_has_permissions rhp ON rhp.permission_id = permissions.id").
		Where("rhp.role_id = ?", roleID).
		Find(&permissions).Error
	return permissions, err
}

// ---------- 3. Custom + pagination: subquery filter, reuse findPaged ----------
// Do NOT re-implement COUNT/LIMIT/OFFSET. Build the base query and hand it to
// findPaged, which applies QueryOptions (sort, conditions, preloads, paging).
func (r *PermissionRepository) FindAllByRoleWithOpts(tx *gorm.DB, roleID uint, opts *QueryOptions) (*PagedResult[models.Permission], error) {
	sub := r.getDB(tx).Table("role_has_permissions").Select("permission_id").Where("role_id = ?", roleID)
	base := r.getDB(tx).Model(&models.Permission{}).Where("permissions.id IN (?)", sub)
	return r.findPaged(base, opts)
}

// ---------- 4. Custom: aggregate into a small struct ----------
type RolePermissionCount struct {
	RoleID uint
	Total  int64
}

func (r *PermissionRepository) CountPerRole(tx *gorm.DB) ([]RolePermissionCount, error) {
	var rows []RolePermissionCount
	err := r.getDB(tx).Table("role_has_permissions").
		Select("role_id, COUNT(*) AS total").
		Group("role_id").
		Scan(&rows).Error
	return rows, err
}

// ---------- 5. Custom: only the columns you need, in ONE query ----------
// Example from the project: UserRoleRepository.FindAccessNames joins
// users → user_has_roles → roles → role_has_permissions → permissions and returns
// []string names, instead of Preload("Roles.Permissions") (5 queries, full rows).

// ============================================================
// N+1 — never query inside a loop
// ============================================================
//
// WRONG (1 query for the list + 1 per row):
//     users, _ := s.repo.User.FindAll(nil)
//     for i := range users {
//         roles, _ := s.repo.UserRole.FindRolesByUserID(users[i].ID) // N extra queries
//         ...
//     }
//
// RIGHT — has-many / many-to-many shown in the response → Preload (1 query per relation level):
//     result, _ := s.repo.User.FindAllWithOpts(nil, &repositories.QueryOptions{Preloads: []string{"Roles"}})
//
// RIGHT — belongs-to used to FILTER or SORT, or you only need a column → JOIN in a custom method:
//     r.getDB(tx).Model(&models.User{}).Joins("JOIN satkers ON satkers.id = users.satker_id").
//         Where("satkers.code = ?", code)
//
// RIGHT — need data for a set of ids → one query with IN, then map in Go:
//     var rows []models.Role
//     r.getDB(tx).Where("id IN ?", ids).Find(&rows)
//
// Preload only what the response actually renders: Preload("Roles.Permissions")
// on a list endpoint loads every permission of every role of every user on the page.
