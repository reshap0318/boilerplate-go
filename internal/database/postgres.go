package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgreSQLConfig holds PostgreSQL configuration.
type PostgreSQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// NewPostgreSQL creates a new PostgreSQL database connection.
func NewPostgreSQL(cfg PostgreSQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), gormConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	log.Println("PostgreSQL connection established")

	configurePool(db, "PostgreSQL")

	return db, nil
}
