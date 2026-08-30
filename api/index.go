package handler

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"job-tracker/internal/app"
	"job-tracker/internal/config"
)

var (
	application *app.App
	once        sync.Once
)

// Handler serves as the Vercel Serverless Function entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		cfg := config.Load()
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		application = app.BuildApp(cfg, logger)
	})

	application.Handler.ServeHTTP(w, r)
}
