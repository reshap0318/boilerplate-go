package repositories

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/helpers"
)

// QueryCondition represents a single WHERE condition.
type QueryCondition struct {
	Column   string      // Column name, e.g. "entry_time"
	Operator string      // SQL operator, e.g. "=", ">=", "<=", "LIKE", "IN"
	Value    interface{} // Condition value
}

// ConditionGroup represents a group of conditions joined by a logic operator.
// Groups are always AND-ed together.
type ConditionGroup struct {
	Logic      string           // "AND" | "OR" — logic between conditions within this group
	Conditions []QueryCondition // Conditions in this group
}

// QueryOptions holds options for querying records.
type QueryOptions struct {
	Page            int              // Page number (default: 1)
	PageSize        int              // Items per page (0/unset = default 10, negative = no pagination, returns all)
	SortBy          string           // Field to sort by
	Order           string           // "ASC" or "DESC" (default: "ASC")
	Preloads        []string         // Relations to preload
	Omits           []string         // Columns to omit from SELECT
	ConditionGroups []ConditionGroup // WHERE condition groups, AND-ed together; each group joins its conditions by group Logic
}

// PagedResult holds paginated query results.
type PagedResult[T any] struct {
	Data       []T   `json:"data"`        // Data records
	Total      int64 `json:"total"`       // Total records
	Page       int   `json:"page"`        // Current page
	PageSize   int   `json:"page_size"`   // Items per page
	TotalPages int   `json:"total_pages"` // Total pages
}

// GetData returns the data slice.
func (p *PagedResult[T]) GetData() interface{} {
	return p.Data
}

// GetMetadata returns the pagination metadata.
func (p *PagedResult[T]) GetMetadata() helpers.PaginationMeta {
	return helpers.PaginationMeta{
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

// GenericRepository provides generic CRUD operations for any model
type GenericRepository[T any] struct {
	DB    *gorm.DB
	Model *T
}

// NewGenericRepository creates a new generic repository
func NewGenericRepository[T any](db *gorm.DB, model *T) *GenericRepository[T] {
	return &GenericRepository[T]{
		DB:    db,
		Model: model,
	}
}

// getDB returns the transaction DB if provided, otherwise the default DB
func (r *GenericRepository[T]) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.DB
}

// identifierPattern allows "column" or "table.column" — nothing that can carry SQL.
var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// allowedOperators is the whitelist for QueryCondition.Operator.
var allowedOperators = map[string]bool{
	"=": true, "!=": true, "<>": true, "<": true, "<=": true, ">": true, ">=": true,
	"LIKE": true, "NOT LIKE": true, "ILIKE": true, "IN": true, "NOT IN": true,
}

// ErrInvalidQueryOption is returned when SortBy, a condition column or an
// operator is not a safe identifier/whitelisted operator. These values are
// concatenated into SQL, so they must never come unchecked from user input.
var ErrInvalidQueryOption = errors.New("invalid query option")

func validateIdentifier(name string) error {
	if !identifierPattern.MatchString(name) {
		return fmt.Errorf("%w: column %q", ErrInvalidQueryOption, name)
	}
	return nil
}

// applyFilters applies sorting, WHERE condition groups and omits — everything
// that shapes WHICH rows are returned. Preloads are applied separately (only on
// the data query) so COUNT(*) never triggers relation loading.
func (r *GenericRepository[T]) applyFilters(db *gorm.DB, opts *QueryOptions) (*gorm.DB, error) {
	if opts == nil {
		return db, nil
	}

	// Sorting (identifier is validated because it is concatenated into SQL)
	if opts.SortBy != "" {
		if err := validateIdentifier(opts.SortBy); err != nil {
			return nil, err
		}
		order := "ASC"
		if strings.ToUpper(opts.Order) == "DESC" {
			order = "DESC"
		}
		db = db.Order(opts.SortBy + " " + order)
	}

	// Condition groups — each group is AND-ed; conditions within a group use group Logic
	for _, group := range opts.ConditionGroups {
		if len(group.Conditions) == 0 {
			continue
		}
		logic := "AND"
		if strings.ToUpper(group.Logic) == "OR" {
			logic = "OR"
		}
		clauses := make([]string, 0, len(group.Conditions))
		args := make([]interface{}, 0, len(group.Conditions))
		for _, cond := range group.Conditions {
			if err := validateIdentifier(cond.Column); err != nil {
				return nil, err
			}
			op := strings.ToUpper(strings.TrimSpace(cond.Operator))
			if !allowedOperators[op] {
				return nil, fmt.Errorf("%w: operator %q", ErrInvalidQueryOption, cond.Operator)
			}
			clauses = append(clauses, cond.Column+" "+op+" ?")
			args = append(args, cond.Value)
		}
		db = db.Where("("+strings.Join(clauses, " "+logic+" ")+")", args...)
	}

	// Omit columns
	if len(opts.Omits) > 0 {
		db = db.Omit(opts.Omits...)
	}

	return db, nil
}

// applyPreloads applies relation preloads (one extra query per relation level,
// never one per row — this is the N+1-safe way to load has-many relations).
func applyPreloads(db *gorm.DB, preloads []string) *gorm.DB {
	for _, preload := range preloads {
		db = db.Preload(preload)
	}
	return db
}

// FindByID finds a record by ID
func (r *GenericRepository[T]) FindByID(tx *gorm.DB, id uint, preloads ...string) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	query := db.Model(&instance).Where("id = ?", id)

	query = applyPreloads(query, preloads)

	if err := query.First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helpers.ErrNotFound
		}
		return nil, err
	}
	return instance, nil
}

// FindByIDWithOpts finds a record by ID with query options
func (r *GenericRepository[T]) FindByIDWithOpts(tx *gorm.DB, id uint, opts *QueryOptions) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	query, err := r.applyFilters(db, opts)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		query = applyPreloads(query, opts.Preloads)
	}

	if err := query.Where("id = ?", id).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helpers.ErrNotFound
		}
		return nil, err
	}
	return instance, nil
}

// Create creates a new record
func (r *GenericRepository[T]) Create(tx *gorm.DB, request *T) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	if err := db.Model(&instance).Create(&request).Error; err != nil {
		return nil, err
	}
	return request, nil
}

// CreateMany creates multiple records
func (r *GenericRepository[T]) CreateMany(tx *gorm.DB, request []T) error {
	db := r.getDB(tx)
	var instance *T
	return db.Model(&instance).Create(&request).Error
}

// Update updates a record by filter
func (r *GenericRepository[T]) Update(tx *gorm.DB, filter *T, update *T) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	if err := db.Model(&instance).Where(filter).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helpers.ErrNotFound
		}
		return nil, err
	}

	if err := db.Model(&instance).Where(filter).Updates(update).Error; err != nil {
		return nil, err
	}

	return instance, nil
}

// UpdateMap updates a record by filter using a map for partial updates (supports zero values)
func (r *GenericRepository[T]) UpdateMap(tx *gorm.DB, filter *T, update map[string]interface{}) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	if err := db.Model(&instance).Where(filter).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helpers.ErrNotFound
		}
		return nil, err
	}

	if err := db.Model(&instance).Where(filter).Updates(update).Error; err != nil {
		return nil, err
	}

	return instance, nil
}

// Delete deletes a record by ID
func (r *GenericRepository[T]) Delete(tx *gorm.DB, id uint) (*T, error) {
	db := r.getDB(tx)
	var instance *T
	if err := db.Model(&instance).Where("id = ?", id).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helpers.ErrNotFound
		}
		return nil, err
	}

	if err := db.Model(&instance).Where("id = ?", id).Delete(&instance).Error; err != nil {
		return nil, err
	}

	return instance, nil
}

// FindAll finds all records
func (r *GenericRepository[T]) FindAll(tx *gorm.DB, preloads ...string) ([]T, error) {
	db := r.getDB(tx)
	var instance *T
	query := db.Model(&instance)

	query = applyPreloads(query, preloads)

	datas := []T{}
	if err := query.Find(&datas).Error; err != nil {
		return nil, err
	}

	return datas, nil
}

// FindAllWithOpts finds all records with query options (supports pagination, sorting, preloads, search).
// opts.PageSize < 0 returns all records without pagination.
func (r *GenericRepository[T]) FindAllWithOpts(tx *gorm.DB, opts *QueryOptions) (*PagedResult[T], error) {
	return r.findPaged(r.getDB(tx).Model(new(T)), opts)
}

// findPaged runs a COUNT + paginated SELECT on a base query (which may already
// carry custom WHERE/JOIN clauses). Custom repository methods that need extra
// conditions the generic QueryOptions cannot express (subqueries, JOINs) build
// their base query and call this instead of re-implementing pagination.
//
// COUNT runs without preloads/ORDER BY; preloads run only for the page of rows
// actually returned.
func (r *GenericRepository[T]) findPaged(base *gorm.DB, opts *QueryOptions) (*PagedResult[T], error) {
	query, err := r.applyFilters(base, opts)
	if err != nil {
		return nil, err
	}

	// Get total count (Session keeps the count from mutating the data query)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}

	// Apply pagination.
	// PageSize == 0 (unset) -> default page size; PageSize < 0 -> no limit (all records); PageSize > 0 -> that size.
	page := 1
	pageSize := 10
	if opts != nil {
		if opts.Page > 0 {
			page = opts.Page
		}
		switch {
		case opts.PageSize < 0:
			pageSize = 0
		case opts.PageSize > 0:
			pageSize = opts.PageSize
		}
	}

	datas := []T{}
	if total > 0 {
		dataQuery := query
		if opts != nil {
			dataQuery = applyPreloads(dataQuery, opts.Preloads)
		}
		if pageSize > 0 {
			dataQuery = dataQuery.Limit(pageSize).Offset((page - 1) * pageSize)
		}
		if err := dataQuery.Find(&datas).Error; err != nil {
			return nil, err
		}
	}

	totalPages := 1
	if pageSize > 0 {
		totalPages = int(total) / pageSize
		if int(total)%pageSize != 0 {
			totalPages++
		}
	}

	return &PagedResult[T]{
		Data:       datas,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// FindByField finds records by filter (note: ignores zero values like 0, false, "")
func (r *GenericRepository[T]) FindByField(tx *gorm.DB, filter *T, preloads ...string) ([]T, error) {
	db := r.getDB(tx)
	var instance *T
	query := db.Model(&instance).Where(filter)

	query = applyPreloads(query, preloads)

	datas := []T{}
	if err := query.Find(&datas).Error; err != nil {
		return nil, err
	}

	return datas, nil
}

// FindByFieldMap finds records by filter map (supports zero values like 0, false, "")
func (r *GenericRepository[T]) FindByFieldMap(tx *gorm.DB, filter map[string]interface{}, preloads ...string) ([]T, error) {
	db := r.getDB(tx)
	var instance *T
	query := db.Model(&instance).Where(filter)

	query = applyPreloads(query, preloads)

	datas := []T{}
	if err := query.Find(&datas).Error; err != nil {
		return nil, err
	}

	return datas, nil
}

// Count counts all records
func (r *GenericRepository[T]) Count(tx *gorm.DB) (int64, error) {
	db := r.getDB(tx)
	var instance *T
	var count int64
	if err := db.Model(&instance).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ExistsWithMap checks if a record exists by filter map (supports zero values like 0, false, "")
func (r *GenericRepository[T]) ExistsWithMap(tx *gorm.DB, filter map[string]interface{}) (bool, error) {
	return r.exists(r.getDB(tx).Model(new(T)).Where(filter))
}

// Exists checks if a record exists by struct filter (note: ignores zero values)
func (r *GenericRepository[T]) Exists(tx *gorm.DB, filter *T) (bool, error) {
	return r.exists(r.getDB(tx).Model(new(T)).Where(filter))
}

// exists runs SELECT 1 ... LIMIT 1, which stops at the first match instead of
// counting every matching row like COUNT(*) does.
func (r *GenericRepository[T]) exists(query *gorm.DB) (bool, error) {
	var found int
	res := query.Select("1").Limit(1).Scan(&found)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
