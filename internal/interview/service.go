package interview

import (
	"context"
	"errors"
)

var (
	ErrInterviewNotFound = errors.New("interview not found")
)

// Service provides business logic operations for interviews.
type Service struct {
	repo Repository
}

// NewService creates a new interview Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, interview *Interview) error {
	return s.repo.Create(ctx, interview)
}

func (s *Service) Get(ctx context.Context, userID, id string) (*Interview, error) {
	i, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	return i, nil
}

func (s *Service) ListByApplication(ctx context.Context, userID, applicationID string) ([]Interview, error) {
	return s.repo.ListByApplicationID(ctx, userID, applicationID)
}

func (s *Service) Update(ctx context.Context, userID, id string, i *Interview) error {
	return s.repo.Update(ctx, userID, id, i)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}
