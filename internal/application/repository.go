package application

import "context"

// UpdateInput represents mutable fields on an application.
type UpdateInput struct {
	Company        *string
	Position       *string
	Location       *string
	JobURL         *string
	SalaryMin      *float64
	SalaryMax      *float64
	SalaryCurrency *string
	Status         *Status
	Notes          *string
}

// Repository defines data persistence methods for job applications.
type Repository interface {
	Create(ctx context.Context, app *Application) error
	GetByID(ctx context.Context, userID, id string) (*Application, error)
	List(ctx context.Context, userID string, filter Filter) ([]Application, int, error)
	Update(ctx context.Context, userID, id string, input UpdateInput) (*Application, error)
	Delete(ctx context.Context, userID, id string) error
	CreateEvent(ctx context.Context, event *Event) error
	ListEvents(ctx context.Context, userID, applicationID string) ([]Event, error)
}
