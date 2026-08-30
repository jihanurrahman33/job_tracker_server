package application

import (
	"errors"
	"net/url"
	"strings"
)

var validStatuses = map[Status]bool{
	StatusApplied:            true,
	StatusScreening:          true,
	StatusInterview:          true,
	StatusTechnicalInterview: true,
	StatusOffer:              true,
	StatusRejected:           true,
	StatusWithdrawn:          true,
	StatusAccepted:           true,
}

// ValidateStatus checks if a given status string is a recognized application status.
func ValidateStatus(s string) (Status, bool) {
	status := Status(strings.ToUpper(strings.TrimSpace(s)))
	if validStatuses[status] {
		return status, true
	}
	return "", false
}

// ValidateApplicationInput validates the fields for creating/updating applications.
func ValidateApplicationInput(company, position, status string, jobURL string, salaryMin, salaryMax *float64) map[string]string {
	errs := make(map[string]string)

	if strings.TrimSpace(company) == "" {
		errs["company"] = "company is required and must not be empty"
	} else if len(company) > 255 {
		errs["company"] = "company must be 255 characters or fewer"
	}

	if strings.TrimSpace(position) == "" {
		errs["position"] = "position is required and must not be empty"
	} else if len(position) > 255 {
		errs["position"] = "position must be 255 characters or fewer"
	}

	if status != "" {
		if _, ok := ValidateStatus(status); !ok {
			errs["status"] = "status must be one of APPLIED, SCREENING, INTERVIEW, TECHNICAL_INTERVIEW, OFFER, REJECTED, WITHDRAWN, ACCEPTED"
		}
	}

	if jobURL != "" {
		if u, err := url.ParseRequestURI(jobURL); err != nil || u.Scheme == "" || u.Host == "" {
			errs["job_url"] = "job_url must be a valid URL"
		}
	}

	if salaryMin != nil && *salaryMin < 0 {
		errs["salary_min"] = "salary_min must be greater than or equal to 0"
	}
	if salaryMax != nil && *salaryMax < 0 {
		errs["salary_max"] = "salary_max must be greater than or equal to 0"
	}
	if salaryMin != nil && salaryMax != nil && *salaryMax < *salaryMin {
		errs["salary_max"] = "salary_max must be greater than or equal to salary_min"
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

var ErrValidationFailed = errors.New("validation failed")
