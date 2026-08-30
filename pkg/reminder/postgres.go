package reminder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"job-tracker/pkg/database"
)

type postgresRepository struct {
	db *database.DB
}

// NewPostgresRepository creates a new PostgreSQL reminder repository.
func NewPostgresRepository(db *database.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, rem *Reminder) error {
	query := `
		INSERT INTO reminders (
			id, user_id, application_id, title, description, remind_at, completed, created_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, NOW()
		)
		RETURNING id, created_at`

	return r.db.QueryRowContext(
		ctx, query,
		rem.UserID, rem.ApplicationID, rem.Title, rem.Description, rem.RemindAt, rem.Completed,
	).Scan(&rem.ID, &rem.CreatedAt)
}

func (r *postgresRepository) GetByID(ctx context.Context, userID, id string) (*Reminder, error) {
	query := `
		SELECT id, user_id, application_id, title, description, remind_at, completed, created_at
		FROM reminders
		WHERE id = $1 AND user_id = $2`

	var rem Reminder
	var appID, desc sql.NullString

	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&rem.ID, &rem.UserID, &appID, &rem.Title, &desc, &rem.RemindAt, &rem.Completed, &rem.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get reminder: %w", err)
	}

	if appID.Valid {
		rem.ApplicationID = &appID.String
	}
	rem.Description = desc.String

	return &rem, nil
}

func (r *postgresRepository) List(ctx context.Context, userID string) ([]Reminder, error) {
	query := `
		SELECT id, user_id, application_id, title, description, remind_at, completed, created_at
		FROM reminders
		WHERE user_id = $1
		ORDER BY remind_at ASC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var rem Reminder
		var appID, desc sql.NullString

		if err := rows.Scan(
			&rem.ID, &rem.UserID, &appID, &rem.Title, &desc, &rem.RemindAt, &rem.Completed, &rem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reminder: %w", err)
		}

		if appID.Valid {
			rem.ApplicationID = &appID.String
		}
		rem.Description = desc.String
		reminders = append(reminders, rem)
	}

	return reminders, nil
}

func (r *postgresRepository) Update(ctx context.Context, userID, id string, rem *Reminder) error {
	query := `
		UPDATE reminders
		SET
			title = $1,
			description = $2,
			remind_at = $3,
			completed = $4
		WHERE id = $5 AND user_id = $6`

	_, err := r.db.ExecContext(
		ctx, query,
		rem.Title, rem.Description, rem.RemindAt, rem.Completed,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update reminder: %w", err)
	}

	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, userID, id string) error {
	query := `DELETE FROM reminders WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *postgresRepository) GetDueReminders(ctx context.Context, maxCount int) ([]Reminder, error) {
	if maxCount <= 0 {
		maxCount = 50
	}

	query := `
		SELECT id, user_id, application_id, title, description, remind_at, completed, created_at
		FROM reminders
		WHERE completed = false AND remind_at <= NOW()
		ORDER BY remind_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, maxCount)
	if err != nil {
		return nil, fmt.Errorf("failed to query due reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var rem Reminder
		var appID, desc sql.NullString

		if err := rows.Scan(
			&rem.ID, &rem.UserID, &appID, &rem.Title, &desc, &rem.RemindAt, &rem.Completed, &rem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan due reminder: %w", err)
		}

		if appID.Valid {
			rem.ApplicationID = &appID.String
		}
		rem.Description = desc.String
		reminders = append(reminders, rem)
	}

	return reminders, nil
}

func (r *postgresRepository) MarkCompleted(ctx context.Context, id string) error {
	query := `UPDATE reminders SET completed = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
