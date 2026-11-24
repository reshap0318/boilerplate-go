package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/reshap0318/go-boilerplate/internal/di"
	"github.com/reshap0318/go-boilerplate/internal/helpers"
	"github.com/reshap0318/go-boilerplate/internal/middleware"
	"github.com/reshap0318/go-boilerplate/internal/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	host := helpers.GetEnv("APP_HOST", "0.0.0.0")
	port := helpers.GetEnv("APP_PORT", "8080")
	trustedProxies := helpers.GetEnv("TRUSTED_PROXIES", "")
	allowedOrigins := helpers.GetEnv("ALLOWED_ORIGINS", "*")

	gin.SetMode(helpers.GetEnv("GIN_MODE", "release"))

	container, err := di.NewContainer()
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer container.Close()

	r := gin.Default()

	// gin trusts EVERY proxy by default, which makes X-Forwarded-For spoofable.
	// Empty TRUSTED_PROXIES therefore means "trust none" (use the socket address).
	var proxies []string
	if trustedProxies != "" {
		proxies = strings.Split(trustedProxies, ",")
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		log.Printf("Warning: failed to set trusted proxies: %v", err)
	}

	r.Use(middleware.RateLimit(container.RateLimiter))
	r.Use(middleware.CORS(allowedOrigins))

	// storage/ holds only public uploads; secrets (keys/, logs/) live outside it, not web-servable.
	r.Static("/storage", "./storage")

	r.NoRoute(func(c *gin.Context) {
		helpers.NotFound(c, "Endpoint not found")
	})

	apiGroup := r.Group("/api")
	protected := apiGroup.Group("")
	protected.Use(middleware.JWTAuth(container.Services))

	routes.RegisterAll(r, apiGroup, protected, container.Handlers, container.Access)

	srv := &http.Server{
		Addr:              host + ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Server starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown: stop accepting new requests, let in-flight ones finish,
	// then the deferred container.Close() releases DB/Redis/logger.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Forced shutdown: %v", err)
	}
	log.Println("Server stopped")
}
