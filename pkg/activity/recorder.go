// Package activity records who did what and when, so an admin can monitor use
// of the service and spot abuse. It never stores request bodies, tokens or
// passwords.
package activity

import (
	"context"
	"database/sql"
	"log/slog"
	"math/rand"
	"time"

	"job-tracker/pkg/database"
)

const (
	retention = "90 days"

	// Sign-in lockout: this many recent failures against one account, or from
	// one address, blocks further attempts until the window passes.
	maxFailuresPerSubject = 5
	maxFailuresPerIP      = 30
	failureWindow         = "15 minutes"

	maxSignupsPerIP = 10
	signupWindow    = "1 hour"
)

// Event is one recorded request.
type Event struct {
	RequestID  string
	UserID     string
	Action     string
	Method     string
	Path       string
	Status     int
	DurationMS int
	IP         string
	UserAgent  string
	Subject    string
}

// Recorder writes events to PostgreSQL. A nil Recorder, or one without a
// database, quietly does nothing.
type Recorder struct {
	db     *database.DB
	logger *slog.Logger
}

// NewRecorder creates a Recorder.
func NewRecorder(db *database.DB, logger *slog.Logger) *Recorder {
	return &Recorder{db: db, logger: logger}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Record stores an event. Failures are logged and swallowed: monitoring must
// never break the request it describes.
func (r *Recorder) Record(ctx context.Context, e Event) {
	if r == nil || r.db == nil {
		return
	}
	// The request has finished, so use a fresh short deadline rather than a
	// context that may already be cancelled.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()

	var userID sql.NullString
	if e.UserID != "" {
		userID = sql.NullString{String: e.UserID, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_logs
			(request_id, user_id, action, method, path, status, duration_ms, ip, user_agent, subject)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''))`,
		clip(e.RequestID, 64), userID, clip(e.Action, 64), clip(e.Method, 10), clip(e.Path, 255),
		e.Status, e.DurationMS, clip(e.IP, 64), clip(e.UserAgent, 255), clip(e.Subject, 64),
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("failed to record activity", slog.String("error", err.Error()))
		}
		return
	}

	// Housekeeping: now and then, drop events past the retention period.
	if rand.Intn(200) == 0 {
		_, _ = r.db.ExecContext(ctx, `DELETE FROM activity_logs WHERE created_at < NOW() - INTERVAL '`+retention+`'`)
	}
}

// LoginBlocked reports whether sign-in attempts are over the limit for this
// account (identified by its hashed email) or for this address.
func (r *Recorder) LoginBlocked(ctx context.Context, ip, subject string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var bySubject, byIP int
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE subject = $2),
			COUNT(*) FILTER (WHERE ip = $1)
		FROM activity_logs
		WHERE action = 'auth.login_failed'
		  AND created_at > NOW() - INTERVAL '`+failureWindow+`'
		  AND (subject = $2 OR ip = $1)`, ip, subject).Scan(&bySubject, &byIP)
	if err != nil {
		return false, err
	}
	return bySubject >= maxFailuresPerSubject || byIP >= maxFailuresPerIP, nil
}

// RegisterBlocked reports whether this address has created too many accounts
// recently.
func (r *Recorder) RegisterBlocked(ctx context.Context, ip string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM activity_logs
		WHERE action = 'auth.register' AND ip = $1
		  AND created_at > NOW() - INTERVAL '`+signupWindow+`'`, ip).Scan(&n)
	if err != nil {
		return false, err
	}
	return n >= maxSignupsPerIP, nil
}
