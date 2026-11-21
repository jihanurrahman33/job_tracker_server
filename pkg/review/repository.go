package review

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"job-tracker/pkg/database"
)

// Review is a user's own review, as returned to that user.
type Review struct {
	ID        string    `json:"id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicReview is a review as shown on the public landing page. It never
// includes the user id or email.
type PublicReview struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// Summary is the aggregate rating across all reviews.
type Summary struct {
	Average      float64        `json:"average"`
	Count        int            `json:"count"`
	Distribution map[string]int `json:"distribution"`
}

// Repository persists reviews.
type Repository interface {
	Upsert(ctx context.Context, userID string, rating int, comment string) (*Review, error)
	GetByUser(ctx context.Context, userID string) (*Review, error)
	DeleteByUser(ctx context.Context, userID string) error
	ListPublic(ctx context.Context, limit int) ([]PublicReview, error)
	Summary(ctx context.Context) (*Summary, error)
}

type postgresRepository struct {
	db *database.DB
}

// NewPostgresRepository creates a PostgreSQL-backed review repository.
func NewPostgresRepository(db *database.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Upsert(ctx context.Context, userID string, rating int, comment string) (*Review, error) {
	query := `
		INSERT INTO reviews (id, user_id, rating, comment)
		VALUES (gen_random_uuid(), $1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, updated_at = NOW()
		RETURNING id, rating, comment, created_at, updated_at`

	var rev Review
	err := r.db.QueryRowContext(ctx, query, userID, rating, comment).Scan(
		&rev.ID, &rev.Rating, &rev.Comment, &rev.CreatedAt, &rev.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save review: %w", err)
	}
	return &rev, nil
}

func (r *postgresRepository) GetByUser(ctx context.Context, userID string) (*Review, error) {
	query := `
		SELECT id, rating, comment, created_at, updated_at
		FROM reviews WHERE user_id = $1`

	var rev Review
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&rev.ID, &rev.Rating, &rev.Comment, &rev.CreatedAt, &rev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get review: %w", err)
	}
	return &rev, nil
}

func (r *postgresRepository) DeleteByUser(ctx context.Context, userID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM reviews WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}
	return nil
}

// ListPublic returns the latest reviews that include a written comment.
func (r *postgresRepository) ListPublic(ctx context.Context, limit int) ([]PublicReview, error) {
	query := `
		SELECT r.id, COALESCE(u.name, ''), r.rating, r.comment, r.created_at
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.comment <> ''
		ORDER BY r.created_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list reviews: %w", err)
	}
	defer rows.Close()

	reviews := []PublicReview{}
	for rows.Next() {
		var rev PublicReview
		var name string
		if err := rows.Scan(&rev.ID, &name, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		rev.Author = DisplayName(name)
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

func (r *postgresRepository) Summary(ctx context.Context) (*Summary, error) {
	s := &Summary{Distribution: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}}

	rows, err := r.db.QueryContext(ctx, `SELECT rating, COUNT(*) FROM reviews GROUP BY rating`)
	if err != nil {
		return nil, fmt.Errorf("failed to summarise reviews: %w", err)
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var rating, n int
		if err := rows.Scan(&rating, &n); err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}
		s.Distribution[fmt.Sprint(rating)] = n
		s.Count += n
		total += rating * n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if s.Count > 0 {
		s.Average = float64(total) / float64(s.Count)
	}
	return s, nil
}

// DisplayName shortens a full name to "First L." so reviews don't expose
// full names. An empty name becomes a neutral label.
func DisplayName(name string) string {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "A JobTracker user"
	case 1:
		return parts[0]
	default:
		last := []rune(parts[len(parts)-1])
		return parts[0] + " " + strings.ToUpper(string(last[0])) + "."
	}
}
