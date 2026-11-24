package services

import (
	"errors"
	"runtime/debug"
	"time"

	"gorm.io/gorm"

	"github.com/reshap0318/go-boilerplate/internal/pkg/email"
	"github.com/reshap0318/go-boilerplate/internal/database"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/repositories"
)

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Expiration time.Duration
	RefreshExp time.Duration
}

// ServicesConfig holds all dependencies for Services.
// Add new dependencies here without changing NewServices signature.
type ServicesConfig struct {
	Repo        *repositories.Repositories
	Redis       *database.RedisCache
	Email       *email.EmailClient
	Logger      *helpers.Logger
}

// Services holds all service dependencies.
type Services struct {
	repo         *repositories.Repositories
	RedisClient  *database.RedisCache
	EmailClient  *email.EmailClient
	Logger       *helpers.Logger
	JWKSManager  *JWKSManager
	Access       *helpers.Access
	cfg          *JWTConfig
}

// NewServices creates and initializes all services.
func NewServices(cfg *ServicesConfig) *Services {
	return &Services{
		repo:        cfg.Repo,
		RedisClient: cfg.Redis,
		EmailClient: cfg.Email,
		Logger:      cfg.Logger,
		cfg: &JWTConfig{
			Expiration: time.Duration(helpers.GetEnvInt("JWT_EXPIRATION", 24)) * time.Hour,
			RefreshExp: time.Duration(helpers.GetEnvInt("JWT_REFRESH_EXPIRATION", 168)) * time.Hour,
		},
	}
}

// runAsync runs fn in a background goroutine for fire-and-forget work (e.g. sending
// email) that must not block the HTTP response. A panic inside fn is recovered and
// logged instead of crashing the whole API process.
//
// There is no queue/retry behind this: if the process stops, pending work is lost.
// Use it only for work where that is acceptable.
func (s *Services) runAsync(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.Logger.LogError(name, "panic in background task: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}

// duplicateAsFieldError turns a unique-constraint violation (gorm.ErrDuplicatedKey,
// enabled by TranslateError in the DB config) into a 422 field error. The
// "exists?" pre-check in services gives a friendly message in the common case;
// this covers the race where two requests insert the same value at once.
func duplicateAsFieldError(err error, field, message string) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return &helpers.FieldError{Field: field, Message: message}
	}
	return err
}
