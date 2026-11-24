package di

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_trans "github.com/go-playground/validator/v10/translations/en"
	"gorm.io/gorm"

	clientEmail "github.com/reshap0318/go-boilerplate/internal/pkg/email"
	"github.com/reshap0318/go-boilerplate/internal/database"
	"github.com/reshap0318/go-boilerplate/internal/handlers"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
	"github.com/reshap0318/go-boilerplate/internal/services"
)

// Container holds all dependencies.
type Container struct {
	DB           *gorm.DB
	Redis        *database.RedisCache
	Access       *helpers.Access
	EmailClient  *clientEmail.EmailClient
	Logger       *helpers.Logger
	RateLimiter  *helpers.RateLimiter
	Repositories *repositories.Repositories
	Services     *services.Services
	Handlers     *handlers.Handlers
}

// Close closes all connections.
func (c *Container) Close() error {
	if c.DB != nil {
		sqlDB, err := c.DB.DB()
		if err != nil {
			return fmt.Errorf("error getting database connection: %w", err)
		}
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("error closing database connection: %w", err)
		}
		log.Println("Database connection closed")
	}

	if c.Redis != nil {
		log.Println("Redis connection closed")
	}

	if c.Logger != nil {
		c.Logger.Close()
		log.Println("Logger closed")
	}

	return nil
}

// NewContainer creates and initializes all dependencies.
func NewContainer() (*Container, error) {
	container := &Container{}

	// Initialize Logger (early, before other components)
	logger, err := helpers.NewLogger(helpers.GetEnv("LOG_DIR", "private/logs"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	container.Logger = logger

	// Determine database connection type (default: mysql)
	dbConnection := helpers.GetEnv("DB_CONNECTION", "mysql")

	// Initialize database based on DB_CONNECTION
	if dbConnection == "postgres" || dbConnection == "postgresql" {
		postgres, err := database.NewPostgreSQL(database.PostgreSQLConfig{
			Host:     helpers.GetEnv("DB_HOST", "127.0.0.1"),
			Port:     helpers.GetEnv("DB_PORT", "5432"),
			User:     helpers.GetEnv("DB_USERNAME", "postgres"),
			Password: helpers.GetEnv("DB_PASSWORD", ""),
			DBName:   helpers.GetEnv("DB_DATABASE", "boilerplate"),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize PostgreSQL: %w", err)
		}
		container.DB = postgres
	} else {
		// Default to MySQL
		mysql, err := database.NewMySQL(database.MySQLConfig{
			Host:     helpers.GetEnv("DB_HOST", "127.0.0.1"),
			Port:     helpers.GetEnv("DB_PORT", "3306"),
			User:     helpers.GetEnv("DB_USERNAME", "root"),
			Password: helpers.GetEnv("DB_PASSWORD", ""),
			DBName:   helpers.GetEnv("DB_DATABASE", "boilerplate"),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize MySQL: %w", err)
		}
		container.DB = mysql
	}

	// Initialize Redis (optional)
	if helpers.GetEnv("REDIS_ENABLED", "false") == "true" {
		redisClient, err := database.NewRedis(database.RedisConfig{
			Host:     helpers.GetEnv("REDIS_HOST", "localhost"),
			Port:     helpers.GetEnv("REDIS_PORT", "6379"),
			User:     helpers.GetEnv("REDIS_USER", "root"),
			Password: helpers.GetEnv("REDIS_PASSWORD", ""),
			DB:       helpers.GetEnvInt("REDIS_DB", 0),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Redis: %w", err)
		}
		container.Redis = database.NewRedisCache(redisClient)
	}

	// Initialize Email Client (optional)
	container.EmailClient = clientEmail.NewEmailClient()

	// Initialize Rate Limiter
	rateLimitRequests := helpers.GetEnvInt("RATE_LIMIT_REQUESTS", 100)
	rateLimitWindow := helpers.GetEnvInt("RATE_LIMIT_WINDOW", 60)
	container.RateLimiter = helpers.NewRateLimiter(rateLimitRequests, rateLimitWindow)
	log.Printf("Rate limiting enabled: %d requests per %d seconds", rateLimitRequests, rateLimitWindow)

	// DB is required for repositories and services
	if container.DB == nil {
		panic("database connection is required but not initialized. Set DB_CONNECTION=mysql or DB_CONNECTION=postgres in your .env file")
	}

	// Always initialize repositories
	repos, err := repositories.NewRepositories(container.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}
	container.Repositories = repos

	// Always initialize services (Redis can be nil)
	container.Services = services.NewServices(&services.ServicesConfig{
		Repo:   container.Repositories,
		Redis:  container.Redis,
		Email:  container.EmailClient,
		Logger: container.Logger,
	})

	// Initialize JWKS Manager
	jwksManager := &services.JWKSManager{}
	passphrase, err := helpers.LoadPassphrase(helpers.GetEnv("JWT_PASSPHRASE_PATH", "private/keys/passphrase"))
	if err != nil {
		return nil, fmt.Errorf("failed to load JWT passphrase: %w", err)
	}
	if err := jwksManager.Initialize(
		helpers.GetEnv("JWT_PRIVATE_KEY_PATH", "private/keys/private.pem"),
		helpers.GetEnv("JWT_PUBLIC_KEY_PATH", "private/keys/public.pem"),
		passphrase,
	); err != nil {
		return nil, fmt.Errorf("failed to initialize JWKS Manager: %w", err)
	}
	container.Services.JWKSManager = jwksManager

	// Initialize Access
	// Access loads role/permission names through the repository (one JOIN query).
	acc := helpers.NewAccess(container.Redis, container.Repositories.UserRole.FindAccessNames)
	container.Access = acc
	container.Services.Access = acc

	// Initialize Validator with translator
	validate := validator.New(validator.WithRequiredStructEnabled())

	// Use JSON tag as field name for validation errors
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	uni := ut.New(en.New(), en.New())
	trans, _ := uni.GetTranslator("en")
	en_trans.RegisterDefaultTranslations(validate, trans)

	// Always initialize handlers
	container.Handlers = handlers.NewHandlers(container.Services, validate, trans)

	return container, nil
}
