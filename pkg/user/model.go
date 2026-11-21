package user

import "time"

// Roles. Admins are created by hand in the database, never through the API,
// so no request can grant itself admin rights.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User represents the user domain model.
type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name"`
	Role         string     `json:"role"`
	Disabled     bool       `json:"disabled"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// IsAdmin reports whether the user is an active admin.
func (u *User) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin && !u.Disabled
}
