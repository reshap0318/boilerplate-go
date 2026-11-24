package database

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQLConfig holds MySQL configuration.
type MySQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// NewMySQL creates a new MySQL database connection.
func NewMySQL(cfg MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=false",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), gormConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	log.Println("MySQL connection established")

	configurePool(db, "MySQL")

	return db, nil
}
