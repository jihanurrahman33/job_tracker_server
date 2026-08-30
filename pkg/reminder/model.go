package reminder

import "time"

// Reminder represents a scheduled reminder for an application or task.
type Reminder struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	ApplicationID *string   `json:"application_id,omitempty"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	RemindAt      time.Time `json:"remind_at"`
	Completed     bool      `json:"completed"`
	CreatedAt     time.Time `json:"created_at"`
}
