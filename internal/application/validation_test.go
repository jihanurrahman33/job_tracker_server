package application_test

import (
	"testing"

	"job-tracker/internal/application"
)

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		input string
		want  application.Status
		valid bool
	}{
		{"APPLIED", application.StatusApplied, true},
		{"applied", application.StatusApplied, true},
		{"INTERVIEW", application.StatusInterview, true},
		{"INVALID_STATUS", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got, ok := application.ValidateStatus(tt.input)
		if ok != tt.valid {
			t.Errorf("ValidateStatus(%q) validity = %v, want %v", tt.input, ok, tt.valid)
		}
		if ok && got != tt.want {
			t.Errorf("ValidateStatus(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestValidateApplicationInput(t *testing.T) {
	minVal := 50000.0
	maxVal := 80000.0
	invalidMax := 40000.0

	tests := []struct {
		name       string
		company    string
		position   string
		status     string
		jobURL     string
		salaryMin  *float64
		salaryMax  *float64
		hasError   bool
		errorField string
	}{
		{
			name:       "valid input",
			company:    "Google",
			position:   "Software Engineer",
			status:     "APPLIED",
			jobURL:     "https://careers.google.com/jobs/123",
			salaryMin:  &minVal,
			salaryMax:  &maxVal,
			hasError:   false,
			errorField: "",
		},
		{
			name:       "missing company",
			company:    "",
			position:   "Software Engineer",
			status:     "APPLIED",
			hasError:   true,
			errorField: "company",
		},
		{
			name:       "missing position",
			company:    "Google",
			position:   "",
			status:     "APPLIED",
			hasError:   true,
			errorField: "position",
		},
		{
			name:       "invalid salary range",
			company:    "Google",
			position:   "Software Engineer",
			salaryMin:  &minVal,
			salaryMax:  &invalidMax,
			hasError:   true,
			errorField: "salary_max",
		},
		{
			name:       "invalid url",
			company:    "Google",
			position:   "Software Engineer",
			jobURL:     "not-a-valid-url",
			hasError:   true,
			errorField: "job_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := application.ValidateApplicationInput(tt.company, tt.position, tt.status, tt.jobURL, tt.salaryMin, tt.salaryMax)
			if tt.hasError {
				if len(errs) == 0 {
					t.Fatalf("expected validation errors, got none")
				}
				if _, exists := errs[tt.errorField]; !exists {
					t.Errorf("expected error for field %q, got errors: %v", tt.errorField, errs)
				}
			} else {
				if len(errs) > 0 {
					t.Fatalf("expected no errors, got: %v", errs)
				}
			}
		})
	}
}
