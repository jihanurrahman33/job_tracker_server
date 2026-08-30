package application

import "time"

// Status represents the lifecycle status of a job application.
type Status string

const (
	StatusApplied            Status = "APPLIED"
	StatusScreening          Status = "SCREENING"
	StatusInterview          Status = "INTERVIEW"
	StatusTechnicalInterview Status = "TECHNICAL_INTERVIEW"
	StatusOffer              Status = "OFFER"
	StatusRejected           Status = "REJECTED"
	StatusWithdrawn          Status = "WITHDRAWN"
	StatusAccepted           Status = "ACCEPTED"
)

// Application represents a job application domain model.
type Application struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Company        string     `json:"company"`
	Position       string     `json:"position"`
	Location       string     `json:"location,omitempty"`
	JobURL         string     `json:"job_url,omitempty"`
	SalaryMin      *float64   `json:"salary_min,omitempty"`
	SalaryMax      *float64   `json:"salary_max,omitempty"`
	SalaryCurrency string     `json:"salary_currency,omitempty"`
	Status         Status     `json:"status"`
	AppliedAt      *time.Time `json:"applied_at,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Event represents an audit / timeline event for application status transitions.
type Event struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	OldStatus     *Status   `json:"old_status"`
	NewStatus     Status    `json:"new_status"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Filter represents query filtering options for applications.
type Filter struct {
	Status   string
	Company  string
	Location string
	From     *time.Time
	To       *time.Time
	SortBy   string
	SortDesc bool
	Page     int
	Limit    int
}
