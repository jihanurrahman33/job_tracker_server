package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"job-tracker/internal/database"
)

var (
	allowedSorts = map[string]string{
		"company":    "company",
		"status":     "status",
		"applied_at": "applied_at",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
)

type postgresRepository struct {
	db *database.DB
}

// NewPostgresRepository creates a new PostgreSQL application repository.
func NewPostgresRepository(db *database.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, app *Application) error {
	query := `
		INSERT INTO applications (
			id, user_id, company, position, location, job_url,
			salary_min, salary_max, salary_currency, status, applied_at, notes,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			NOW(), NOW()
		)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(
		ctx, query,
		app.UserID, app.Company, app.Position, app.Location, app.JobURL,
		app.SalaryMin, app.SalaryMax, app.SalaryCurrency, app.Status, app.AppliedAt, app.Notes,
	).Scan(&app.ID, &app.CreatedAt, &app.UpdatedAt)
}

func (r *postgresRepository) GetByID(ctx context.Context, userID, id string) (*Application, error) {
	query := `
		SELECT
			id, user_id, company, position, location, job_url,
			salary_min, salary_max, salary_currency, status, applied_at, notes,
			created_at, updated_at
		FROM applications
		WHERE id = $1 AND user_id = $2`

	var app Application
	var location, jobURL, salaryCurrency, notes sql.NullString

	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&app.ID, &app.UserID, &app.Company, &app.Position, &location, &jobURL,
		&app.SalaryMin, &app.SalaryMax, &salaryCurrency, &app.Status, &app.AppliedAt, &notes,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	app.Location = location.String
	app.JobURL = jobURL.String
	app.SalaryCurrency = salaryCurrency.String
	app.Notes = notes.String

	return &app, nil
}

func (r *postgresRepository) List(ctx context.Context, userID string, filter Filter) ([]Application, int, error) {
	var conditions []string
	var args []any
	argID := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argID))
	args = append(args, userID)
	argID++

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argID))
		args = append(args, filter.Status)
		argID++
	}

	if filter.Company != "" {
		conditions = append(conditions, fmt.Sprintf("company ILIKE $%d", argID))
		args = append(args, "%"+filter.Company+"%")
		argID++
	}

	if filter.Location != "" {
		conditions = append(conditions, fmt.Sprintf("location ILIKE $%d", argID))
		args = append(args, "%"+filter.Location+"%")
		argID++
	}

	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("applied_at >= $%d", argID))
		args = append(args, *filter.From)
		argID++
	}

	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("applied_at <= $%d", argID))
		args = append(args, *filter.To)
		argID++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM applications %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count applications: %w", err)
	}

	sortCol := "created_at"
	if col, ok := allowedSorts[filter.SortBy]; ok {
		sortCol = col
	}

	sortDir := "ASC"
	if filter.SortDesc {
		sortDir = "DESC"
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT
			id, user_id, company, position, location, job_url,
			salary_min, salary_max, salary_currency, status, applied_at, notes,
			created_at, updated_at
		FROM applications
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		whereClause, sortCol, sortDir, argID, argID+1,
	)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list applications: %w", err)
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var app Application
		var location, jobURL, salaryCurrency, notes sql.NullString

		if err := rows.Scan(
			&app.ID, &app.UserID, &app.Company, &app.Position, &location, &jobURL,
			&app.SalaryMin, &app.SalaryMax, &salaryCurrency, &app.Status, &app.AppliedAt, &notes,
			&app.CreatedAt, &app.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan application: %w", err)
		}

		app.Location = location.String
		app.JobURL = jobURL.String
		app.SalaryCurrency = salaryCurrency.String
		app.Notes = notes.String

		apps = append(apps, app)
	}

	return apps, total, nil
}

func (r *postgresRepository) Update(ctx context.Context, userID, id string, input UpdateInput) (*Application, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	current, err := r.getByIDTx(ctx, tx, userID, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}

	oldStatus := current.Status
	statusChanged := false

	if input.Company != nil {
		current.Company = *input.Company
	}
	if input.Position != nil {
		current.Position = *input.Position
	}
	if input.Location != nil {
		current.Location = *input.Location
	}
	if input.JobURL != nil {
		current.JobURL = *input.JobURL
	}
	if input.SalaryMin != nil {
		current.SalaryMin = input.SalaryMin
	}
	if input.SalaryMax != nil {
		current.SalaryMax = input.SalaryMax
	}
	if input.SalaryCurrency != nil {
		current.SalaryCurrency = *input.SalaryCurrency
	}
	if input.Status != nil && *input.Status != oldStatus {
		current.Status = *input.Status
		statusChanged = true
	}
	if input.Notes != nil {
		current.Notes = *input.Notes
	}

	query := `
		UPDATE applications SET
			company = $1,
			position = $2,
			location = $3,
			job_url = $4,
			salary_min = $5,
			salary_max = $6,
			salary_currency = $7,
			status = $8,
			notes = $9,
			updated_at = NOW()
		WHERE id = $10 AND user_id = $11
		RETURNING updated_at`

	err = tx.QueryRowContext(
		ctx, query,
		current.Company, current.Position, current.Location, current.JobURL,
		current.SalaryMin, current.SalaryMax, current.SalaryCurrency, current.Status, current.Notes,
		id, userID,
	).Scan(&current.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update application: %w", err)
	}

	// Insert event atomically inside the transaction if status changed
	if statusChanged {
		eventNote := "Status updated to " + string(current.Status)
		if input.Notes != nil && *input.Notes != "" {
			eventNote = *input.Notes
		}
		eventQuery := `
			INSERT INTO application_events (id, application_id, old_status, new_status, note, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())`
		if _, err := tx.ExecContext(ctx, eventQuery, id, oldStatus, current.Status, eventNote); err != nil {
			return nil, fmt.Errorf("failed to insert status event in transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return current, nil
}

func (r *postgresRepository) getByIDTx(ctx context.Context, tx *sql.Tx, userID, id string) (*Application, error) {
	query := `
		SELECT
			id, user_id, company, position, location, job_url,
			salary_min, salary_max, salary_currency, status, applied_at, notes,
			created_at, updated_at
		FROM applications
		WHERE id = $1 AND user_id = $2 FOR UPDATE`

	var app Application
	var location, jobURL, salaryCurrency, notes sql.NullString

	err := tx.QueryRowContext(ctx, query, id, userID).Scan(
		&app.ID, &app.UserID, &app.Company, &app.Position, &location, &jobURL,
		&app.SalaryMin, &app.SalaryMax, &salaryCurrency, &app.Status, &app.AppliedAt, &notes,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lock and get application: %w", err)
	}

	app.Location = location.String
	app.JobURL = jobURL.String
	app.SalaryCurrency = salaryCurrency.String
	app.Notes = notes.String

	return &app, nil
}

func (r *postgresRepository) Delete(ctx context.Context, userID, id string) error {
	query := `DELETE FROM applications WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
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

func (r *postgresRepository) CreateEvent(ctx context.Context, event *Event) error {
	query := `
		INSERT INTO application_events (id, application_id, old_status, new_status, note, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
		RETURNING id, created_at`

	return r.db.QueryRowContext(ctx, query, event.ApplicationID, event.OldStatus, event.NewStatus, event.Note).
		Scan(&event.ID, &event.CreatedAt)
}

func (r *postgresRepository) ListEvents(ctx context.Context, userID, applicationID string) ([]Event, error) {
	query := `
		SELECT e.id, e.application_id, e.old_status, e.new_status, e.note, e.created_at
		FROM application_events e
		JOIN applications a ON a.id = e.application_id
		WHERE e.application_id = $1 AND a.user_id = $2
		ORDER BY e.created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, applicationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var oldStatus, note sql.NullString

		if err := rows.Scan(&e.ID, &e.ApplicationID, &oldStatus, &e.NewStatus, &note, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if oldStatus.Valid {
			s := Status(oldStatus.String)
			e.OldStatus = &s
		}
		e.Note = note.String
		events = append(events, e)
	}

	return events, nil
}
