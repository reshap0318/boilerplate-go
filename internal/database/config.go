package database

import (
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// gormConfig is shared by every driver.
//   - TranslateError: unique/foreign-key violations come back as gorm.ErrDuplicatedKey /
//     gorm.ErrForeignKeyViolated instead of driver-specific errors.
//   - SkipDefaultTransaction: GORM otherwise wraps every single write in its own
//     BEGIN/COMMIT. Multi-step writes in this project already use TxManager, so the
//     implicit transaction only adds two round-trips per INSERT/UPDATE/DELETE.
func gormConfig() *gorm.Config {
	return &gorm.Config{
		Logger:                 gormLogger,
		TranslateError:         true,
		SkipDefaultTransaction: true,
	}
}

// envInt reads an integer env var, returning def when it is unset or invalid.
// (Same as helpers.GetEnvInt — the database package can't import helpers,
// because helpers already imports database.)
func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// configurePool applies connection pool limits from env.
// DB_CONN_MAX_LIFETIME_MIN / DB_CONN_MAX_IDLE_TIME_MIN recycle connections before
// the server (or a proxy/load balancer) silently drops them.
func configurePool(db *gorm.DB, driver string) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	maxOpen := envInt("DB_MAX_OPEN_CONNS", 25)
	maxIdle := envInt("DB_MAX_IDLE_CONNS", 10)
	maxLifetime := envInt("DB_CONN_MAX_LIFETIME_MIN", 30)
	maxIdleTime := envInt("DB_CONN_MAX_IDLE_TIME_MIN", 5)

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(maxIdleTime) * time.Minute)
	log.Printf("%s connection pool: max_open=%d, max_idle=%d, max_lifetime=%dm, max_idle_time=%dm",
		driver, maxOpen, maxIdle, maxLifetime, maxIdleTime)
}
