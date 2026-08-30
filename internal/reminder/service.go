package reminder

import (
	"context"
	"errors"
)

var (
	ErrReminderNotFound = errors.New("reminder not found")
)

// Service provides business logic operations for reminders.
type Service struct {
	repo Repository
}

// NewService creates a new reminder Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, r *Reminder) error {
	return s.repo.Create(ctx, r)
}

func (s *Service) Get(ctx context.Context, userID, id string) (*Reminder, error) {
	r, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrReminderNotFound
	}
	return r, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Reminder, error) {
	return s.repo.List(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID, id string, r *Reminder) error {
	return s.repo.Update(ctx, userID, id, r)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}
