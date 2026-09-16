package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/abuamar142/auth-service/internal/config"
	"github.com/abuamar142/auth-service/internal/db"
	"github.com/abuamar142/auth-service/internal/handlers"
	"github.com/abuamar142/auth-service/internal/middleware"
	"github.com/abuamar142/auth-service/internal/services"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/abuamar142/auth-service/docs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// @title           Auth Service API
// @version         1.0
// @description     Central authentication microservice — SSO for all abuamar.online projects.
// @termsOfService  https://abuamar.online/terms

// @contact.name   Abu Amar
// @contact.email  abuamar.albadawi@gmail.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      auth.abuamar.online
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer {token}"

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key
// @description Enter your API key

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Load config
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	// Connect to database
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	log.Println("migrations applied successfully")

	// Initialize services
	authSvc := services.NewAuthService(pool, cfg.JWTSecret, cfg.BcryptCost)

	// Initialize handlers
	healthH := handlers.NewHealthHandler()
	authH := handlers.NewAuthHandler(authSvc)
	apiKeyH := handlers.NewAPIKeyHandler(authSvc)

	// Setup router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	// Health (unversioned, load balancers need it)
	r.Get("/api/health", healthH.ServeHTTP)

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public — rate limited
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimitAuth())
			r.Post("/auth/register", authH.Register)
			r.Post("/auth/login", authH.Login)
			r.Post("/auth/refresh", authH.Refresh)
		})

		// Auth-protected
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Post("/auth/logout", authH.Logout)
			r.Get("/auth/me", authH.Me)

			// API key management
			r.Get("/api-keys", apiKeyH.List)
			r.Post("/api-keys", apiKeyH.Create)
			r.Delete("/api-keys/{id}", apiKeyH.Delete)
		})
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("auth service starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func runMigrations(databaseURL string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, databaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

var allowedOrigins = map[string]bool{
	"https://abuamar.online":      true,
	"https://dev.abuamar.online":   true,
	"https://tambangan.abuamar.online":  true,
	"https://dev.tambangan.abuamar.online": true,
	"https://asyaikhoni.abuamar.online": true,
	"https://auth.abuamar.online":  true,
	"http://localhost:5173":        true, // local dev
	"http://localhost:3000":        true, // local dev CMS
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
