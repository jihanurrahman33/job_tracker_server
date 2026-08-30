package interview

import "time"

// Type represents the type of interview.
type Type string

const (
	TypePhoneScreen  Type = "PHONE_SCREEN"
	TypeHR           Type = "HR"
	TypeTechnical    Type = "TECHNICAL"
	TypeBehavioral   Type = "BEHAVIORAL"
	TypeSystemDesign Type = "SYSTEM_DESIGN"
	TypeFinal        Type = "FINAL"
	TypeOther        Type = "OTHER"
)

// Interview represents an interview scheduled for a job application.
type Interview struct {
	ID              string    `json:"id"`
	ApplicationID   string    `json:"application_id"`
	Type            Type      `json:"type"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes *int      `json:"duration_minutes,omitempty"`
	Location        string    `json:"location,omitempty"`
	MeetingURL      string    `json:"meeting_url,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
