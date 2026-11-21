package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"job-tracker/pkg/middleware"
	"job-tracker/pkg/response"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
	maxNameLen     = 100
	maxEmailLen    = 254
)

// AttemptLimiter tells the handler when a source has made too many recent
// attempts. It is backed by the activity log, so the limit holds across
// serverless instances rather than per process.
type AttemptLimiter interface {
	LoginBlocked(ctx context.Context, ip, subject string) (bool, error)
	RegisterBlocked(ctx context.Context, ip string) (bool, error)
}

// Handler processes incoming HTTP requests for authentication.
type Handler struct {
	service *Service
	limiter AttemptLimiter
}

// NewHandler creates a new auth HTTP Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SetLimiter enables lockout of repeated failed sign-ins and of mass sign-ups.
func (h *Handler) SetLimiter(l AttemptLimiter) {
	h.limiter = l
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User  any    `json:"user"`
	Token string `json:"token"`
}

func tooManyAttempts(w http.ResponseWriter, msg string) {
	w.Header().Set("Retry-After", "900")
	response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", msg, nil)
}

// validEmail accepts a plain address such as "a@b.co", not "Name <a@b.co>".
func validEmail(email string) bool {
	if email == "" || len(email) > maxEmailLen {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// Register handles user registration (POST /api/v1/auth/register).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	req.Email = NormalizeEmail(req.Email)
	req.Name = strings.TrimSpace(req.Name)

	valErrs := make(map[string]string)
	if req.Email == "" {
		valErrs["email"] = "email is required"
	} else if !validEmail(req.Email) {
		valErrs["email"] = "email must be a valid address"
	}
	if utf8.RuneCountInString(req.Name) > maxNameLen {
		valErrs["name"] = "name must be 100 characters or fewer"
	}
	if req.Password == "" {
		valErrs["password"] = "password is required"
	} else if utf8.RuneCountInString(req.Password) < minPasswordLen {
		valErrs["password"] = "password must be at least 8 characters"
	} else if len(req.Password) > maxPasswordLen {
		valErrs["password"] = "password must be 128 characters or fewer"
	}

	if len(valErrs) > 0 {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", valErrs)
		return
	}

	if h.limiter != nil {
		if blocked, err := h.limiter.RegisterBlocked(r.Context(), middleware.ClientIP(r)); err == nil && blocked {
			tooManyAttempts(w, "Too many accounts created from this network. Try again later.")
			return
		}
	}

	user, token, err := h.service.Register(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			response.Error(w, http.StatusConflict, response.ErrCodeConflict, "Email already registered", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to register user", nil)
		return
	}

	middleware.SetUserID(r.Context(), user.ID)
	response.JSON(w, http.StatusCreated, authResponse{
		User:  user,
		Token: token,
	})
}

// Login handles user login (POST /api/v1/auth/login).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	req.Email = NormalizeEmail(req.Email)
	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", map[string]string{
			"email":    "email is required",
			"password": "password is required",
		})
		return
	}

	// The activity log keeps only a hash of the address, which is enough to
	// count repeated failures against one account.
	subject := middleware.SubjectHash(req.Email)
	middleware.SetSubject(r.Context(), subject)

	if h.limiter != nil {
		if blocked, err := h.limiter.LoginBlocked(r.Context(), middleware.ClientIP(r), subject); err == nil && blocked {
			tooManyAttempts(w, "Too many failed sign-in attempts. Try again in 15 minutes.")
			return
		}
	}

	user, token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Invalid email or password", nil)
		case errors.Is(err, ErrAccountDisabled):
			response.Error(w, http.StatusForbidden, response.ErrCodeForbidden, "This account has been disabled", nil)
		default:
			response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to authenticate", nil)
		}
		return
	}

	middleware.SetUserID(r.Context(), user.ID)
	response.JSON(w, http.StatusOK, authResponse{
		User:  user,
		Token: token,
	})
}

// Logout handles user logout (POST /api/v1/auth/logout).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := ExtractBearerToken(r)
	if token != "" {
		_ = h.service.Logout(r.Context(), token)
	}
	response.NoContent(w)
}

// ValidateTokenOrSession satisfies the middleware.Authenticator interface.
func (h *Handler) ValidateTokenOrSession(r *http.Request) (string, error) {
	token := ExtractBearerToken(r)
	if token == "" {
		return "", errors.New("missing Authorization header or token")
	}

	return h.service.ValidateToken(r.Context(), token)
}

// ExtractBearerToken reads the session token from the Authorization header, or
// from X-Auth-Token. A token in the URL is deliberately not accepted: URLs end
// up in server logs, browser history and Referer headers.
func ExtractBearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		var token string
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 && !strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[0])
		}

		token = strings.Trim(token, "\"'")
		if token != "" {
			return token
		}
	}

	if custom := strings.TrimSpace(r.Header.Get("X-Auth-Token")); custom != "" {
		return strings.Trim(custom, "\"'")
	}

	return ""
}
