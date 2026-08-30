package statistics_test

import (
	"context"
	"testing"
	"time"

	"job-tracker/internal/application"
	"job-tracker/internal/statistics"
)

type mockAppRepo struct {
	apps []application.Application
}

func (m *mockAppRepo) Create(ctx context.Context, app *application.Application) error {
	m.apps = append(m.apps, *app)
	return nil
}

func (m *mockAppRepo) GetByID(ctx context.Context, userID, id string) (*application.Application, error) {
	for _, a := range m.apps {
		if a.ID == id && a.UserID == userID {
			return &a, nil
		}
	}
	return nil, nil
}

func (m *mockAppRepo) List(ctx context.Context, userID string, filter application.Filter) ([]application.Application, int, error) {
	var userApps []application.Application
	for _, a := range m.apps {
		if a.UserID == userID {
			userApps = append(userApps, a)
		}
	}
	return userApps, len(userApps), nil
}

func (m *mockAppRepo) Update(ctx context.Context, userID, id string, input application.UpdateInput) (*application.Application, error) {
	return nil, nil
}

func (m *mockAppRepo) Delete(ctx context.Context, userID, id string) error {
	return nil
}

func (m *mockAppRepo) CreateEvent(ctx context.Context, event *application.Event) error {
	return nil
}

func (m *mockAppRepo) ListEvents(ctx context.Context, userID, applicationID string) ([]application.Event, error) {
	return nil, nil
}

func TestStatisticsCalculation(t *testing.T) {
	now := time.Now()
	repo := &mockAppRepo{
		apps: []application.Application{
			{ID: "1", UserID: "user-1", Status: application.StatusApplied, CreatedAt: now},
			{ID: "2", UserID: "user-1", Status: application.StatusApplied, CreatedAt: now},
			{ID: "3", UserID: "user-1", Status: application.StatusScreening, CreatedAt: now},
			{ID: "4", UserID: "user-1", Status: application.StatusInterview, CreatedAt: now},
			{ID: "5", UserID: "user-1", Status: application.StatusTechnicalInterview, CreatedAt: now},
			{ID: "6", UserID: "user-1", Status: application.StatusOffer, CreatedAt: now},
			{ID: "7", UserID: "user-1", Status: application.StatusRejected, CreatedAt: now},
			{ID: "8", UserID: "user-1", Status: application.StatusWithdrawn, CreatedAt: now},
			{ID: "9", UserID: "user-1", Status: application.StatusAccepted, CreatedAt: now},
			{ID: "10", UserID: "user-1", Status: application.StatusApplied, CreatedAt: now},
		},
	}

	service := statistics.NewService(repo)
	stats, err := service.Get(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalApplications != 10 {
		t.Errorf("expected 10 total applications, got %d", stats.TotalApplications)
	}

	// 3 APPLIED, 7 non-applied -> response rate = 7/10 = 0.70
	if stats.ResponseRate != 0.70 {
		t.Errorf("expected response_rate 0.70, got %f", stats.ResponseRate)
	}

	// 2 interviews (INTERVIEW, TECHNICAL_INTERVIEW) -> 2/10 = 0.20
	if stats.InterviewRate != 0.20 {
		t.Errorf("expected interview_rate 0.20, got %f", stats.InterviewRate)
	}

	// 2 offers (OFFER, ACCEPTED) -> 2/10 = 0.20
	if stats.OfferRate != 0.20 {
		t.Errorf("expected offer_rate 0.20, got %f", stats.OfferRate)
	}
}
