package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"job-tracker/pkg/response"
)

// Handler processes incoming HTTP requests for authentication.
type Handler struct {
	service *Service
}

// NewHandler creates a new auth HTTP Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

// Register handles user registration (POST /api/v1/auth/register).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)

	valErrs := make(map[string]string)
	if req.Email == "" {
		valErrs["email"] = "email is required"
	}
	if req.Password == "" {
		valErrs["password"] = "password is required"
	} else if len(req.Password) < 6 {
		valErrs["password"] = "password must be at least 6 characters"
	}

	if len(valErrs) > 0 {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", valErrs)
		return
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

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", map[string]string{
			"email":    "email is required",
			"password": "password is required",
		})
		return
	}

	user, token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Invalid email or password", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to authenticate", nil)
		return
	}

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

// ExtractBearerToken flexibly extracts bearer token from Authorization header or fallbacks.
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

	// Fallback header: X-Auth-Token
	if custom := strings.TrimSpace(r.Header.Get("X-Auth-Token")); custom != "" {
		return strings.Trim(custom, "\"'")
	}

	// Fallback query parameter: ?token=
	if qToken := strings.TrimSpace(r.URL.Query().Get("token")); qToken != "" {
		return strings.Trim(qToken, "\"'")
	}

	return ""
}
