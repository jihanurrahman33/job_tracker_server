package user

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

// NewPostgresRepository creates a new PostgreSQL repository for users.
func NewPostgresRepository(db *database.DB) Repository {
	return &postgresRepository{db: db}
}

const userColumns = `id, email, password_hash, name, role, disabled, last_login_at, created_at, updated_at`

func scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	var u User
	var name sql.NullString
	var lastLogin sql.NullTime
	if err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &name, &u.Role, &u.Disabled, &lastLogin, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, err
	}
	u.Name = name.String
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	return &u, nil
}

func (r *postgresRepository) Create(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
		RETURNING id, role, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query, u.Email, u.PasswordHash, u.Name).
		Scan(&u.ID, &u.Role, &u.CreatedAt, &u.UpdatedAt)
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return u, nil
}

func (r *postgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(email) = LOWER($1)`, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return u, nil
}

// TouchLogin stamps the user's last successful sign-in. It is optional: the
// auth service uses it when the repository provides it.
func (r *postgresRepository) TouchLogin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, id)
	return err
}
