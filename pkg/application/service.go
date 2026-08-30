package application

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("application not found")
)

// CreateInput holds the fields needed to create a new application.
type CreateInput struct {
	UserID         string
	Company        string
	Position       string
	Location       string
	JobURL         string
	SalaryMin      *float64
	SalaryMax      *float64
	SalaryCurrency string
	Status         Status
	AppliedAt      *time.Time
	Notes          string
}

// Service manages business operations for job applications.
type Service struct {
	repo Repository
}

// NewService creates a new application Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Application, error) {
	if input.Status == "" {
		input.Status = StatusApplied
	}

	app := &Application{
		UserID:         input.UserID,
		Company:        input.Company,
		Position:       input.Position,
		Location:       input.Location,
		JobURL:         input.JobURL,
		SalaryMin:      input.SalaryMin,
		SalaryMax:      input.SalaryMax,
		SalaryCurrency: input.SalaryCurrency,
		Status:         input.Status,
		AppliedAt:      input.AppliedAt,
		Notes:          input.Notes,
	}

	if err := s.repo.Create(ctx, app); err != nil {
		return nil, err
	}

	// Record initial status event
	_ = s.repo.CreateEvent(ctx, &Event{
		ApplicationID: app.ID,
		OldStatus:     nil,
		NewStatus:     app.Status,
		Note:          "Application created",
	})

	return app, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (*Application, error) {
	app, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrNotFound
	}
	return app, nil
}

func (s *Service) List(ctx context.Context, userID string, filter Filter) ([]Application, int, error) {
	return s.repo.List(ctx, userID, filter)
}

func (s *Service) Update(ctx context.Context, userID, id string, input UpdateInput) (*Application, error) {
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrNotFound
	}

	updated, err := s.repo.Update(ctx, userID, id, input)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *Service) ListEvents(ctx context.Context, userID, applicationID string) ([]Event, error) {
	return s.repo.ListEvents(ctx, userID, applicationID)
}
