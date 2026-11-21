package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"job-tracker/pkg/activity"
	"job-tracker/pkg/admin"
	"job-tracker/pkg/application"
	"job-tracker/pkg/auth"
	"job-tracker/pkg/config"
	"job-tracker/pkg/database"
	"job-tracker/pkg/interview"
	"job-tracker/pkg/keepalive"
	"job-tracker/pkg/middleware"
	"job-tracker/pkg/reminder"
	"job-tracker/pkg/response"
	"job-tracker/pkg/review"
	"job-tracker/pkg/statistics"
	"job-tracker/pkg/user"
)

// App encapsulates dependencies and the HTTP handler.
type App struct {
	Handler   http.Handler
	DB        *database.DB
	Config    *config.Config
	Logger    *slog.Logger
	Reminder  *reminder.Worker
	KeepAlive *keepalive.Pinger
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
	reviewRepo := review.NewPostgresRepository(db)
	adminRepo := admin.NewRepository(db)
	recorder := activity.NewRecorder(db, logger)

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

	// Keep-Alive Anti-Sleep Worker
	var keepAlivePinger *keepalive.Pinger
	if cfg.KeepAliveEnabled && cfg.KeepAliveURL != "" {
		keepAlivePinger = keepalive.NewPinger(cfg.KeepAliveURL, cfg.KeepAliveInterval, logger)
	}

	// Handlers
	authHandler := auth.NewHandler(authService)
	authHandler.SetLimiter(recorder)
	adminHandler := admin.NewHandler(adminRepo)
	userHandler := user.NewHandler(userService)
	appHandler := application.NewHandler(appService)
	interviewHandler := interview.NewHandler(interviewService)
	reminderHandler := reminder.NewHandler(reminderService)
	statsHandler := statistics.NewHandler(statsService)
	reviewHandler := review.NewHandler(reviewRepo)

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

	// Public reviews (aggregate rating + latest written reviews)
	mux.HandleFunc("GET /api/v1/reviews", reviewHandler.List)

	// Protected routes helper
	authMiddleware := middleware.Authenticate(authHandler)

	wrapAuth := func(handlerFunc http.HandlerFunc) http.Handler {
		return authMiddleware(handlerFunc)
	}

	// Admin routes: signed in AND an active admin, checked against the
	// database on every request.
	adminOnly := middleware.RequireAdmin(adminRepo.IsAdmin)
	wrapAdmin := func(handlerFunc http.HandlerFunc) http.Handler {
		return authMiddleware(adminOnly(handlerFunc))
	}
	mux.Handle("GET /api/v1/admin/overview", wrapAdmin(adminHandler.Overview))
	mux.Handle("GET /api/v1/admin/users", wrapAdmin(adminHandler.Users))
	mux.Handle("PATCH /api/v1/admin/users/{id}", wrapAdmin(adminHandler.UpdateUser))
	mux.Handle("GET /api/v1/admin/activity", wrapAdmin(adminHandler.Activity))
	mux.Handle("GET /api/v1/admin/reviews", wrapAdmin(adminHandler.Reviews))
	mux.Handle("DELETE /api/v1/admin/reviews/{id}", wrapAdmin(adminHandler.DeleteReview))

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

	// The signed-in user's own review
	mux.Handle("GET /api/v1/reviews/me", wrapAuth(reviewHandler.GetMine))
	mux.Handle("PUT /api/v1/reviews/me", wrapAuth(reviewHandler.SaveMine))
	mux.Handle("DELETE /api/v1/reviews/me", wrapAuth(reviewHandler.DeleteMine))

	// CORS options
	corsOpts := middleware.DefaultCORSOptions()
	if len(cfg.CORSAllowedOrigins) > 0 {
		corsOpts.AllowedOrigins = cfg.CORSAllowedOrigins
	}

	// Rate limiter (token-bucket)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	// Global middleware stack, outermost first:
	// CORS -> RequestID -> SecurityHeaders -> MaxBody -> Logging -> Activity -> RateLimit -> Recovery -> mux
	// Activity sits outside RateLimit so blocked requests are recorded too.
	var handler http.Handler = mux
	handler = middleware.Recovery(logger)(handler)
	handler = rateLimiter.Limit()(handler)
	handler = activity.Middleware(recorder)(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.MaxBodyBytes(1 << 20)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.CORS(corsOpts)(handler)

	return &App{
		Handler:   handler,
		DB:        db,
		Config:    cfg,
		Logger:    logger,
		Reminder:  reminderWorker,
		KeepAlive: keepAlivePinger,
	}
}
