package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"job-tracker/pkg/app"
	"job-tracker/pkg/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("starting job tracker server",
		slog.String("environment", cfg.Environment),
		slog.String("port", cfg.Port),
	)

	// Graceful shutdown context
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.BuildApp(cfg, logger)
	if application.DB != nil {
		defer application.DB.Close()
	}

	// Start background reminder worker if available
	if application.Reminder != nil {
		go application.Reminder.Start(shutdownCtx, 10*time.Second)
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      application.Handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logger.Info("server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed to start", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-shutdownCtx.Done()
	logger.Info("shutting down gracefully...")

	shutdownTimeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelTimeout()

	if err := server.Shutdown(shutdownTimeoutCtx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	} else {
		logger.Info("server stopped gracefully")
	}
}
