package repositories

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/models"
)

// dryRunDB builds SQL without a database connection.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:pass@tcp(127.0.0.1:1)/x",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestApplyFiltersRejectsUnsafeInput(t *testing.T) {
	repo := NewGenericRepository(dryRunDB(t), &models.User{})
	cases := []*QueryOptions{
		{SortBy: "id; DROP TABLE users"},
		{SortBy: "(SELECT 1)"},
		{ConditionGroups: []ConditionGroup{{Conditions: []QueryCondition{{Column: "id = 1 OR 1", Operator: "=", Value: 1}}}}},
		{ConditionGroups: []ConditionGroup{{Conditions: []QueryCondition{{Column: "id", Operator: "= 1 OR 1 =", Value: 1}}}}},
	}
	for i, opts := range cases {
		if _, err := repo.applyFilters(repo.DB.Model(&models.User{}), opts); !errors.Is(err, ErrInvalidQueryOption) {
			t.Errorf("case %d: expected ErrInvalidQueryOption, got %v", i, err)
		}
	}
}

func TestApplyFiltersGroupsArePrecedenceSafe(t *testing.T) {
	repo := NewGenericRepository(dryRunDB(t), &models.User{})
	opts := &QueryOptions{
		SortBy: "users.id",
		Order:  "desc",
		ConditionGroups: []ConditionGroup{
			{Logic: "OR", Conditions: []QueryCondition{
				{Column: "name", Operator: "like", Value: "%a%"},
				{Column: "email", Operator: "LIKE", Value: "%a%"},
			}},
			{Conditions: []QueryCondition{{Column: "id", Operator: "IN", Value: []uint{1, 2}}}},
		},
	}
	q, err := repo.applyFilters(repo.DB.Model(&models.User{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	sql := q.Find(&[]models.User{}).Statement.SQL.String()
	for _, want := range []string{"(name LIKE ? OR email LIKE ?)", "(id IN (?,?))", "ORDER BY users.id DESC", "`users`.`deleted_at` IS NULL"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL %q missing %q", sql, want)
		}
	}
}
