package interview

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

// NewPostgresRepository creates a new PostgreSQL interview repository.
func NewPostgresRepository(db *database.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, i *Interview) error {
	query := `
		INSERT INTO interviews (
			id, application_id, type, scheduled_at, duration_minutes,
			location, meeting_url, notes, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4,
			$5, $6, $7, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(
		ctx, query,
		i.ApplicationID, i.Type, i.ScheduledAt, i.DurationMinutes,
		i.Location, i.MeetingURL, i.Notes,
	).Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
}

func (r *postgresRepository) GetByID(ctx context.Context, userID, id string) (*Interview, error) {
	query := `
		SELECT
			i.id, i.application_id, i.type, i.scheduled_at, i.duration_minutes,
			i.location, i.meeting_url, i.notes, i.created_at, i.updated_at
		FROM interviews i
		JOIN applications a ON a.id = i.application_id
		WHERE i.id = $1 AND a.user_id = $2`

	var i Interview
	var loc, meetURL, notes sql.NullString

	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&i.ID, &i.ApplicationID, &i.Type, &i.ScheduledAt, &i.DurationMinutes,
		&loc, &meetURL, &notes, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get interview: %w", err)
	}

	i.Location = loc.String
	i.MeetingURL = meetURL.String
	i.Notes = notes.String

	return &i, nil
}

func (r *postgresRepository) ListByApplicationID(ctx context.Context, userID, applicationID string) ([]Interview, error) {
	query := `
		SELECT
			i.id, i.application_id, i.type, i.scheduled_at, i.duration_minutes,
			i.location, i.meeting_url, i.notes, i.created_at, i.updated_at
		FROM interviews i
		JOIN applications a ON a.id = i.application_id
		WHERE i.application_id = $1 AND a.user_id = $2
		ORDER BY i.scheduled_at ASC`

	rows, err := r.db.QueryContext(ctx, query, applicationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list interviews: %w", err)
	}
	defer rows.Close()

	var interviews []Interview
	for rows.Next() {
		var i Interview
		var loc, meetURL, notes sql.NullString

		if err := rows.Scan(
			&i.ID, &i.ApplicationID, &i.Type, &i.ScheduledAt, &i.DurationMinutes,
			&loc, &meetURL, &notes, &i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan interview: %w", err)
		}

		i.Location = loc.String
		i.MeetingURL = meetURL.String
		i.Notes = notes.String

		interviews = append(interviews, i)
	}

	return interviews, nil
}

func (r *postgresRepository) Update(ctx context.Context, userID, id string, i *Interview) error {
	query := `
		UPDATE interviews i
		SET
			type = $1,
			scheduled_at = $2,
			duration_minutes = $3,
			location = $4,
			meeting_url = $5,
			notes = $6,
			updated_at = NOW()
		FROM applications a
		WHERE i.id = $7 AND i.application_id = a.id AND a.user_id = $8
		RETURNING i.updated_at`

	return r.db.QueryRowContext(
		ctx, query,
		i.Type, i.ScheduledAt, i.DurationMinutes, i.Location, i.MeetingURL, i.Notes,
		id, userID,
	).Scan(&i.UpdatedAt)
}

func (r *postgresRepository) Delete(ctx context.Context, userID, id string) error {
	query := `
		DELETE FROM interviews i
		USING applications a
		WHERE i.id = $1 AND i.application_id = a.id AND a.user_id = $2`

	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete interview: %w", err)
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
