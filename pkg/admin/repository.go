// Package admin provides the read and moderation queries behind the admin
// dashboard. Every handler in it sits behind middleware.RequireAdmin.
package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"job-tracker/pkg/database"
)

// ErrNotFound means the target of an admin action does not exist, or may not
// be acted on (for example another admin).
var ErrNotFound = errors.New("not found")

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// IsAdmin reports whether the user is an active admin. It is the check
// RequireAdmin runs on every admin request.
func (r *Repository) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND role = 'admin' AND disabled = FALSE)`,
		userID).Scan(&ok)
	return ok, err
}

type DayCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type Overview struct {
	Totals struct {
		Users        int     `json:"users"`
		Applications int     `json:"applications"`
		Interviews   int     `json:"interviews"`
		Reminders    int     `json:"reminders"`
		Reviews      int     `json:"reviews"`
		AvgRating    float64 `json:"average_rating"`
	} `json:"totals"`
	Users struct {
		New7d     int `json:"new_7d"`
		New30d    int `json:"new_30d"`
		Disabled  int `json:"disabled"`
		Active24h int `json:"active_24h"`
		Active7d  int `json:"active_7d"`
	} `json:"users"`
	Last24h struct {
		Events       int `json:"events"`
		Logins       int `json:"logins"`
		FailedLogins int `json:"failed_logins"`
		Blocked      int `json:"blocked"`
		ClientErrors int `json:"client_errors"`
		ServerErrors int `json:"server_errors"`
	} `json:"last_24h"`
	SignupsByDay         []DayCount     `json:"signups_by_day"`
	EventsByDay          []DayCount     `json:"events_by_day"`
	ApplicationsByStatus map[string]int `json:"applications_by_status"`
	RecentEvents         []ActivityItem `json:"recent_events"`
}

func (r *Repository) Overview(ctx context.Context) (*Overview, error) {
	o := &Overview{ApplicationsByStatus: map[string]int{}}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM applications),
			(SELECT COUNT(*) FROM interviews),
			(SELECT COUNT(*) FROM reminders),
			(SELECT COUNT(*) FROM reviews),
			(SELECT COALESCE(AVG(rating), 0) FROM reviews),
			(SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '7 days'),
			(SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '30 days'),
			(SELECT COUNT(*) FROM users WHERE disabled),
			(SELECT COUNT(DISTINCT user_id) FROM activity_logs WHERE user_id IS NOT NULL AND created_at > NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(DISTINCT user_id) FROM activity_logs WHERE user_id IS NOT NULL AND created_at > NOW() - INTERVAL '7 days'),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours' AND action = 'auth.login'),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours' AND action = 'auth.login_failed'),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours'
				AND action IN ('security.rate_limited', 'auth.login_blocked', 'auth.register_blocked')),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours' AND status BETWEEN 400 AND 499),
			(SELECT COUNT(*) FROM activity_logs WHERE created_at > NOW() - INTERVAL '24 hours' AND status >= 500)`,
	).Scan(
		&o.Totals.Users, &o.Totals.Applications, &o.Totals.Interviews, &o.Totals.Reminders, &o.Totals.Reviews, &o.Totals.AvgRating,
		&o.Users.New7d, &o.Users.New30d, &o.Users.Disabled, &o.Users.Active24h, &o.Users.Active7d,
		&o.Last24h.Events, &o.Last24h.Logins, &o.Last24h.FailedLogins, &o.Last24h.Blocked,
		&o.Last24h.ClientErrors, &o.Last24h.ServerErrors,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load overview: %w", err)
	}

	if o.SignupsByDay, err = r.perDay(ctx, "users"); err != nil {
		return nil, err
	}
	if o.EventsByDay, err = r.perDay(ctx, "activity_logs"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM applications GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("failed to load status counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		o.ApplicationsByStatus[status] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	events, _, err := r.ListActivity(ctx, ActivityFilter{Limit: 8})
	if err != nil {
		return nil, err
	}
	o.RecentEvents = events
	return o, nil
}

// perDay counts rows created on each of the last 14 days, zero-filled. The
// table name is chosen from a fixed list, never from input.
func (r *Repository) perDay(ctx context.Context, table string) ([]DayCount, error) {
	if table != "users" && table != "activity_logs" {
		return nil, errors.New("unsupported table")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_char(d, 'YYYY-MM-DD'), COALESCE(c.n, 0)
		FROM generate_series((CURRENT_DATE - 13)::timestamp, CURRENT_DATE::timestamp, '1 day') d
		LEFT JOIN (
			SELECT created_at::date AS day, COUNT(*) AS n
			FROM `+table+`
			WHERE created_at >= CURRENT_DATE - 13
			GROUP BY 1
		) c ON c.day = d::date
		ORDER BY d`)
	if err != nil {
		return nil, fmt.Errorf("failed to load daily counts: %w", err)
	}
	defer rows.Close()

	out := []DayCount{}
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type UserRow struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Disabled     bool       `json:"disabled"`
	Applications int        `json:"applications"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// likePattern escapes LIKE wildcards so a search for "100%" means those
// characters, not "anything".
func likePattern(s string) string {
	return "%" + escapeLike(s) + "%"
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Repository) ListUsers(ctx context.Context, search string, page, limit int) ([]UserRow, int, error) {
	where := ""
	args := []any{}
	if search != "" {
		where = `WHERE u.email ILIKE $1 OR COALESCE(u.name, '') ILIKE $1`
		args = append(args, likePattern(search))
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT u.id, COALESCE(u.name, ''), u.email, u.role, u.disabled, u.last_login_at, u.created_at,
			(SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id)
		FROM users u %s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	out := []UserRow{}
	for rows.Next() {
		var u UserRow
		var last sql.NullTime
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Disabled, &last, &u.CreatedAt, &u.Applications); err != nil {
			return nil, 0, err
		}
		if last.Valid {
			u.LastLoginAt = &last.Time
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// SetDisabled suspends or restores a regular user. Admins cannot be disabled
// here, which also stops an admin locking themselves out. Disabling ends the
// user's sessions at once.
func (r *Repository) SetDisabled(ctx context.Context, id string, disabled bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET disabled = $2, updated_at = NOW() WHERE id = $1 AND role <> 'admin'`, id, disabled)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if disabled {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, id); err != nil {
			return fmt.Errorf("failed to end sessions: %w", err)
		}
	}
	return nil
}

type ActivityItem struct {
	ID         int64     `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Action     string    `json:"action"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int       `json:"duration_ms"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	RequestID  string    `json:"request_id"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	UserName   string    `json:"user_name"`
}

type ActivityFilter struct {
	Action    string // exact action name, or a prefix ending in "." such as "auth."
	UserID    string
	IP        string
	MinStatus int
	Page      int
	Limit     int
}

func (r *Repository) ListActivity(ctx context.Context, f ActivityFilter) ([]ActivityItem, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 25
	}

	conds := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Action != "" {
		if strings.HasSuffix(f.Action, ".") {
			add("l.action LIKE $%d", escapeLike(strings.TrimSuffix(f.Action, "."))+".%")
		} else {
			add("l.action = $%d", f.Action)
		}
	}
	if f.UserID != "" {
		add("l.user_id = $%d", f.UserID)
	}
	if f.IP != "" {
		add("l.ip = $%d", f.IP)
	}
	if f.MinStatus > 0 {
		add("l.status >= $%d", f.MinStatus)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_logs l `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count activity: %w", err)
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT l.id, l.created_at, l.action, l.method, l.path, l.status, l.duration_ms,
			COALESCE(l.ip, ''), COALESCE(l.user_agent, ''), COALESCE(l.request_id, ''),
			COALESCE(u.id::text, ''), COALESCE(u.email, ''), COALESCE(u.name, '')
		FROM activity_logs l
		LEFT JOIN users u ON u.id = l.user_id
		%s
		ORDER BY l.id DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list activity: %w", err)
	}
	defer rows.Close()

	out := []ActivityItem{}
	for rows.Next() {
		var a ActivityItem
		if err := rows.Scan(&a.ID, &a.CreatedAt, &a.Action, &a.Method, &a.Path, &a.Status, &a.DurationMS,
			&a.IP, &a.UserAgent, &a.RequestID, &a.UserID, &a.UserEmail, &a.UserName); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

type ReviewRow struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) ListReviews(ctx context.Context, page, limit int) ([]ReviewRow, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count reviews: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, COALESCE(u.name, ''), u.email, r.rating, r.comment, r.created_at
		FROM reviews r JOIN users u ON u.id = r.user_id
		ORDER BY r.created_at DESC LIMIT $1 OFFSET $2`, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list reviews: %w", err)
	}
	defer rows.Close()

	out := []ReviewRow{}
	for rows.Next() {
		var v ReviewRow
		if err := rows.Scan(&v.ID, &v.Author, &v.Email, &v.Rating, &v.Comment, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r *Repository) DeleteReview(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
