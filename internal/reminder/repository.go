package reminder

import "context"

// Repository defines storage operations for reminders.
type Repository interface {
	Create(ctx context.Context, r *Reminder) error
	GetByID(ctx context.Context, userID, id string) (*Reminder, error)
	List(ctx context.Context, userID string) ([]Reminder, error)
	Update(ctx context.Context, userID, id string, r *Reminder) error
	Delete(ctx context.Context, userID, id string) error
	GetDueReminders(ctx context.Context, maxCount int) ([]Reminder, error)
	MarkCompleted(ctx context.Context, id string) error
}
