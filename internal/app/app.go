package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"job-tracker/internal/application"
	"job-tracker/internal/auth"
	"job-tracker/internal/config"
	"job-tracker/internal/database"
	"job-tracker/internal/interview"
	"job-tracker/internal/middleware"
	"job-tracker/internal/reminder"
	"job-tracker/internal/response"
	"job-tracker/internal/statistics"
	"job-tracker/internal/user"
)

// App encapsulates dependencies and the HTTP handler.
type App struct {
	Handler  http.Handler
	DB       *database.DB
	Config   *config.Config
	Logger   *slog.Logger
	Reminder *reminder.Worker
}

// BuildApp initializes configurations, database connection, repositories, services, and routes.
func BuildApp(cfg *config.Config, logger *slog.Logger) *App {
	if cfg == nil {
		cfg = config.Load()
	}
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	var db *database.DB
	var sessionStore auth.SessionStore = auth.NewMemorySessionStore(24 * time.Hour)

	poolConfig := database.PoolConfig{
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}

	connectedDB, err := database.ConnectWithRetry("postgres", cfg.DatabaseURL, poolConfig, 3, 2*time.Second)
	if err != nil {
		logger.Warn("could not connect to PostgreSQL database on startup (will retry on requests)", slog.String("error", err.Error()))
	} else {
		db = connectedDB
		logger.Info("connected to PostgreSQL successfully")

		// Run migrations
		migDir := database.FindMigrationsDir()
		if _, err := os.Stat(migDir); err == nil {
			migCtx, migCancel := context.WithTimeout(context.Background(), 20*time.Second)
			if err := db.RunMigrations(migCtx, migDir); err != nil {
				logger.Error("failed to apply migrations", slog.String("error", err.Error()))
			} else {
				logger.Info("database migrations applied successfully", slog.String("directory", migDir))
			}
			migCancel()
		}

		sessionStore = auth.NewPostgresSessionStore(db, 24*time.Hour)
	}

	// Repositories
	userRepo := user.NewPostgresRepository(db)
	appRepo := application.NewPostgresRepository(db)
	interviewRepo := interview.NewPostgresRepository(db)
	reminderRepo := reminder.NewPostgresRepository(db)

	// Services
	authService := auth.NewService(userRepo, sessionStore)
	userService := user.NewService(userRepo)
	appService := application.NewService(appRepo)
	interviewService := interview.NewService(interviewRepo)
	reminderService := reminder.NewService(reminderRepo)
	statsService := statistics.NewService(appRepo)

	// Reminder Worker
	var reminderWorker *reminder.Worker
	if db != nil {
		reminderWorker = reminder.NewWorker(reminderRepo, logger)
	}

	// Handlers
	authHandler := auth.NewHandler(authService)
	userHandler := user.NewHandler(userService)
	appHandler := application.NewHandler(appService)
	interviewHandler := interview.NewHandler(interviewService)
	reminderHandler := reminder.NewHandler(reminderService)
	statsHandler := statistics.NewHandler(statsService)

	// Router setup
	mux := http.NewServeMux()

	// Root welcome route
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]any{
			"name":    "Job Application Tracker API",
			"version": "1.0",
			"status":  "healthy",
			"endpoints": map[string]string{
				"health":  "/health",
				"healthz": "/healthz",
				"ready":   "/ready",
			},
		})
	})

	// Public health check routes
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /healthz", healthHandler)

	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			response.Error(w, http.StatusServiceUnavailable, response.ErrCodeInternalError, "Database handle not initialized", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			response.Error(w, http.StatusServiceUnavailable, response.ErrCodeInternalError, "Database unreachable", nil)
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "connected"})
	})

	// Auth public routes
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Protected routes helper
	authMiddleware := middleware.Authenticate(authHandler)

	wrapAuth := func(handlerFunc http.HandlerFunc) http.Handler {
		return authMiddleware(handlerFunc)
	}

	// User / Profile route
	mux.Handle("GET /api/v1/me", wrapAuth(userHandler.Me))

	// Applications routes
	mux.Handle("POST /api/v1/applications", wrapAuth(appHandler.Create))
	mux.Handle("GET /api/v1/applications", wrapAuth(appHandler.List))
	mux.Handle("GET /api/v1/applications/{id}", wrapAuth(appHandler.Get))
	mux.Handle("PATCH /api/v1/applications/{id}", wrapAuth(appHandler.Update))
	mux.Handle("DELETE /api/v1/applications/{id}", wrapAuth(appHandler.Delete))
	mux.Handle("GET /api/v1/applications/{id}/events", wrapAuth(appHandler.Events))

	// Interviews routes
	mux.Handle("POST /api/v1/applications/{id}/interviews", wrapAuth(interviewHandler.Create))
	mux.Handle("GET /api/v1/applications/{id}/interviews", wrapAuth(interviewHandler.ListByApplication))
	mux.Handle("GET /api/v1/interviews/{id}", wrapAuth(interviewHandler.Get))
	mux.Handle("PATCH /api/v1/interviews/{id}", wrapAuth(interviewHandler.Update))
	mux.Handle("DELETE /api/v1/interviews/{id}", wrapAuth(interviewHandler.Delete))

	// Reminders routes
	mux.Handle("POST /api/v1/reminders", wrapAuth(reminderHandler.Create))
	mux.Handle("GET /api/v1/reminders", wrapAuth(reminderHandler.List))
	mux.Handle("PATCH /api/v1/reminders/{id}", wrapAuth(reminderHandler.Update))
	mux.Handle("DELETE /api/v1/reminders/{id}", wrapAuth(reminderHandler.Delete))

	// Statistics routes
	mux.Handle("GET /api/v1/statistics", wrapAuth(statsHandler.Get))

	// Global middleware stack: RequestID -> Logging -> Recovery -> mux
	var handler http.Handler = mux
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.RequestID(handler)

	return &App{
		Handler:  handler,
		DB:       db,
		Config:   cfg,
		Logger:   logger,
		Reminder: reminderWorker,
	}
}
