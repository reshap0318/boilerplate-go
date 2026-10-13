package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/reshap0318/go-boilerplate/cmd/migration/seeders"
	"github.com/reshap0318/go-boilerplate/internal/database"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbConnection := helpers.GetEnv("DB_CONNECTION", "mysql")
	dbHost := helpers.GetEnv("DB_HOST", "127.0.0.1")
	dbPort := helpers.GetEnv("DB_PORT", "3306")
	dbUser := helpers.GetEnv("DB_USERNAME", "root")
	dbPassword := helpers.GetEnv("DB_PASSWORD", "")
	dbName := helpers.GetEnv("DB_DATABASE", "boilerplate")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		log.Fatal("Database configuration is incomplete. Please set DB_HOST, DB_PORT, DB_USERNAME, and DB_DATABASE")
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		runMigration(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName, "up")
	case "down":
		runMigration(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName, "down")
	case "seed":
		gormDB := initGORM(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
		runSeed(gormDB)
	case "sync-rbac":
		gormDB := initGORM(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
		runSyncRBAC(gormDB)
	case "refresh":
		runMigration(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName, "reset")
		gormDB := initGORM(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
		runSeed(gormDB)
	case "status":
		runStatus(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: go run cmd/migration/main.go <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up         Apply all migrations (create/update tables)")
	fmt.Println("  down       Rollback last migration")
	fmt.Println("  seed       Insert default data")
	fmt.Println("  sync-rbac  Sync permissions and roles defined in code with the database")
	fmt.Println("  refresh    Rollback all, re-apply migrations, and seed data")
	fmt.Println("  status     Show migration status")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run cmd/migration/main.go up")
	fmt.Println("  go run cmd/migration/main.go down")
	fmt.Println("  go run cmd/migration/main.go seed")
	fmt.Println("  go run cmd/migration/main.go sync-rbac")
	fmt.Println("  go run cmd/migration/main.go refresh")
	fmt.Println("  go run cmd/migration/main.go status")
}

func getSQLDB(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName string) (*sql.DB, goose.Dialect, error) {
	var dialect goose.Dialect

	if dbConnection == "postgres" || dbConnection == "postgresql" {
		dialect = goose.DialectPostgres
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword, dbName)
		db, err := sql.Open("pgx", dsn)
		return db, dialect, err
	}

	dialect = goose.DialectMySQL
	cfg := mysql.NewConfig()
	cfg.User = dbUser
	cfg.Passwd = dbPassword
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", dbHost, dbPort)
	cfg.DBName = dbName
	cfg.ParseTime = true
	cfg.MultiStatements = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	return db, dialect, err
}

func createProvider(db *sql.DB, dialect goose.Dialect) (*goose.Provider, error) {
	migrations, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(dialect, db, migrations)
}

func runMigration(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName, command string) {
	db, dialect, err := getSQLDB(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	provider, err := createProvider(db, dialect)
	if err != nil {
		log.Fatalf("Failed to create migration provider: %v", err)
	}

	ctx := context.Background()

	switch command {
	case "up":
		fmt.Println("Running migrations...")
		results, err := provider.Up(ctx)
		if err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Printf("✓ Applied %d migration(s) successfully!\n", len(results))
		for _, r := range results {
			fmt.Printf("  → %s (%s)\n", r.Source.Path, r.Duration)
		}

	case "down":
		fmt.Println("Rolling back last migration...")
		result, err := provider.Down(ctx)
		if err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		fmt.Printf("✓ Rolled back %s successfully!\n", result.Source.Path)

	case "reset":
		fmt.Println("Rolling back all migrations...")
		results, err := provider.DownTo(ctx, 0)
		if err != nil {
			log.Fatalf("Reset failed: %v", err)
		}
		fmt.Printf("✓ Rolled back %d migration(s)\n", len(results))

		fmt.Println("Re-applying migrations...")
		results, err = provider.Up(ctx)
		if err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Printf("✓ Applied %d migration(s) successfully!\n", len(results))
	}
}

func runStatus(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName string) {
	db, dialect, err := getSQLDB(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	provider, err := createProvider(db, dialect)
	if err != nil {
		log.Fatalf("Failed to create migration provider: %v", err)
	}

	ctx := context.Background()

	current, err := provider.GetDBVersion(ctx)
	if err != nil {
		log.Fatalf("Failed to get DB version: %v", err)
	}

	migrations := provider.ListSources()

	fmt.Println("Migration status:")
	fmt.Printf("Current version: %d\n\n", current)

	for _, m := range migrations {
		status := "Pending"
		if m.Version <= current {
			status = "Applied"
		}
		fmt.Printf("  [%s] %s (v%d)\n", status, m.Path, m.Version)
	}
}

func runSeed(db *gorm.DB) {
	fmt.Println("\nSeeding default data...")

	permIDs := seeders.SeedPermissions(db)
	roleIDs := seeders.SeedRoles(db)
	userEmails := seeders.SeedUsers(db)
	seeders.SeedRolePermissions(db, roleIDs, permIDs)
	seeders.SeedUserRoles(db, userEmails, roleIDs)

	fmt.Println("\n✅ Seeding completed!")
}

func runSyncRBAC(db *gorm.DB) {
	fmt.Println("\nSyncing RBAC with the database...")

	if err := seeders.SyncRBAC(db); err != nil {
		log.Fatalf("RBAC sync failed: %v", err)
	}

	fmt.Println("\n✅ RBAC sync completed!")
}

func initGORM(dbConnection, dbHost, dbPort, dbUser, dbPassword, dbName string) *gorm.DB {
	var db *gorm.DB
	var err error

	if dbConnection == "postgres" || dbConnection == "postgresql" {
		db, err = database.NewPostgreSQL(database.PostgreSQLConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
		})
	} else {
		db, err = database.NewMySQL(database.MySQLConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
		})
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	return db
}
