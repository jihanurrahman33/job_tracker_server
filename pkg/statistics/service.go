package statistics

import (
	"context"
	"math"

	"job-tracker/pkg/application"
)

// Statistics holds aggregated metrics for a user's job applications.
type Statistics struct {
	TotalApplications int            `json:"total_applications"`
	ByStatus          map[string]int `json:"by_status"`
	ResponseRate      float64        `json:"response_rate"`
	InterviewRate     float64        `json:"interview_rate"`
	OfferRate         float64        `json:"offer_rate"`
}

// Service aggregates statistics for a user.
type Service struct {
	appRepo application.Repository
}

// NewService creates a new statistics Service.
func NewService(appRepo application.Repository) *Service {
	return &Service{appRepo: appRepo}
}

func (s *Service) Get(ctx context.Context, userID string) (*Statistics, error) {
	apps, _, err := s.appRepo.List(ctx, userID, application.Filter{Limit: 1000})
	if err != nil {
		return nil, err
	}

	byStatus := map[string]int{
		string(application.StatusApplied):            0,
		string(application.StatusScreening):          0,
		string(application.StatusInterview):          0,
		string(application.StatusTechnicalInterview): 0,
		string(application.StatusOffer):              0,
		string(application.StatusRejected):           0,
		string(application.StatusWithdrawn):          0,
		string(application.StatusAccepted):           0,
	}

	total := len(apps)
	nonApplied := 0
	interviews := 0
	offers := 0

	for _, app := range apps {
		byStatus[string(app.Status)]++

		if app.Status != application.StatusApplied {
			nonApplied++
		}
		if app.Status == application.StatusInterview || app.Status == application.StatusTechnicalInterview {
			interviews++
		}
		if app.Status == application.StatusOffer || app.Status == application.StatusAccepted {
			offers++
		}
	}

	var responseRate, interviewRate, offerRate float64
	if total > 0 {
		responseRate = roundToTwoDecimals(float64(nonApplied) / float64(total))
		interviewRate = roundToTwoDecimals(float64(interviews) / float64(total))
		offerRate = roundToTwoDecimals(float64(offers) / float64(total))
	}

	return &Statistics{
		TotalApplications: total,
		ByStatus:          byStatus,
		ResponseRate:      responseRate,
		InterviewRate:     interviewRate,
		OfferRate:         offerRate,
	}, nil
}

func roundToTwoDecimals(val float64) float64 {
	return math.Round(val*100) / 100
}
