package reminder

import (
	"context"
	"log/slog"
	"time"
)

// Worker periodically checks and processes scheduled reminders in the background.
type Worker struct {
	repo   Repository
	logger *slog.Logger
}

// NewWorker creates a new reminder background worker.
func NewWorker(repo Repository, logger *slog.Logger) *Worker {
	return &Worker{
		repo:   repo,
		logger: logger,
	}
}

// Start launches the background worker loop until context is canceled.
func (w *Worker) Start(ctx context.Context, interval time.Duration) {
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}

	w.logger.Info("reminder background worker started", slog.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("reminder background worker stopped")
			return
		case <-ticker.C:
			w.processDueReminders(ctx)
		}
	}
}

func (w *Worker) processDueReminders(ctx context.Context) {
	reminders, err := w.repo.GetDueReminders(ctx, 25)
	if err != nil {
		w.logger.Error("failed to fetch due reminders", slog.String("error", err.Error()))
		return
	}

	for _, rem := range reminders {
		w.logger.Info("processing reminder notification",
			slog.String("reminder_id", rem.ID),
			slog.String("user_id", rem.UserID),
			slog.String("title", rem.Title),
			slog.Time("remind_at", rem.RemindAt),
		)

		if err := w.repo.MarkCompleted(ctx, rem.ID); err != nil {
			w.logger.Error("failed to mark reminder as completed",
				slog.String("reminder_id", rem.ID),
				slog.String("error", err.Error()),
			)
		}
	}
}
