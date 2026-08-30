package interview

import "context"

// Repository defines database operations for interviews.
type Repository interface {
	Create(ctx context.Context, userID string, interview *Interview) error
	GetByID(ctx context.Context, userID, id string) (*Interview, error)
	ListByApplicationID(ctx context.Context, userID, applicationID string) ([]Interview, error)
	Update(ctx context.Context, userID, id string, interview *Interview) error
	Delete(ctx context.Context, userID, id string) error
}
