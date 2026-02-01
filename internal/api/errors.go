// Package api provides error handling utilities for the REST API.
package api

import (
	"errors"
	"net/http"

	"github.com/chronos/chronos/internal/models"
)

// APIError represents a structured API error.
type APIError struct {
	HTTPStatus int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Common API error codes.
const (
	ErrCodeInvalidJSON      = "INVALID_JSON"
	ErrCodeValidation       = "VALIDATION_ERROR"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeAlreadyExists    = "ALREADY_EXISTS"
	ErrCodeStoreError       = "STORE_ERROR"
	ErrCodeExecutionError   = "EXECUTION_ERROR"
	ErrCodeInvalidTimeout   = "INVALID_TIMEOUT"
	ErrCodeInvalidVersion   = "INVALID_VERSION"
	ErrCodeNotLeader        = "NOT_LEADER"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeRateLimited      = "RATE_LIMITED"
	ErrCodeInternalError    = "INTERNAL_ERROR"
)

// Predefined API errors.
var (
	ErrInvalidJSON = &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       ErrCodeInvalidJSON,
		Message:    "Invalid JSON body",
	}
	ErrInvalidTimeout = &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       ErrCodeInvalidTimeout,
		Message:    "Invalid timeout format",
	}
	ErrInvalidVersion = &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       ErrCodeInvalidVersion,
		Message:    "Version must be a number",
	}
	ErrJobNotFound = &APIError{
		HTTPStatus: http.StatusNotFound,
		Code:       ErrCodeNotFound,
		Message:    "Job not found",
	}
	ErrExecutionNotFound = &APIError{
		HTTPStatus: http.StatusNotFound,
		Code:       ErrCodeNotFound,
		Message:    "Execution not found",
	}
	ErrJobVersionNotFound = &APIError{
		HTTPStatus: http.StatusNotFound,
		Code:       ErrCodeNotFound,
		Message:    "Job version not found",
	}
	ErrJobAlreadyExists = &APIError{
		HTTPStatus: http.StatusConflict,
		Code:       ErrCodeAlreadyExists,
		Message:    "Job already exists",
	}
	ErrNotLeader = &APIError{
		HTTPStatus: http.StatusServiceUnavailable,
		Code:       ErrCodeNotLeader,
		Message:    "This node is not the cluster leader",
	}
	ErrStoreError = &APIError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       ErrCodeStoreError,
		Message:    "Storage operation failed",
	}
	ErrInternalError = &APIError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       ErrCodeInternalError,
		Message:    "Internal server error",
	}
)

// NewValidationError creates a validation error with a custom message.
func NewValidationError(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       ErrCodeValidation,
		Message:    message,
	}
}

// NewExecutionError creates an execution error with a custom message.
func NewExecutionError(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       ErrCodeExecutionError,
		Message:    message,
	}
}

// MapDomainError maps domain/model errors to API errors.
func MapDomainError(err error) *APIError {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, models.ErrJobNotFound):
		return ErrJobNotFound
	case errors.Is(err, models.ErrExecutionNotFound):
		return ErrExecutionNotFound
	case errors.Is(err, models.ErrJobVersionNotFound):
		return ErrJobVersionNotFound
	case errors.Is(err, models.ErrJobAlreadyExists):
		return ErrJobAlreadyExists
	case errors.Is(err, models.ErrNotLeader):
		return ErrNotLeader
	case errors.Is(err, models.ErrJobNameRequired):
		return NewValidationError(err.Error())
	case errors.Is(err, models.ErrScheduleRequired):
		return NewValidationError(err.Error())
	case errors.Is(err, models.ErrWebhookRequired):
		return NewValidationError(err.Error())
	case errors.Is(err, models.ErrWebhookURLRequired):
		return NewValidationError(err.Error())
	case errors.Is(err, models.ErrInvalidCronExpr):
		return NewValidationError(err.Error())
	case errors.Is(err, models.ErrInvalidTimezone):
		return NewValidationError(err.Error())
	default:
		// Log unexpected errors in production
		return &APIError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       ErrCodeInternalError,
			Message:    "An unexpected error occurred",
		}
	}
}

// WriteAPIError writes an API error response.
func (h *Handler) WriteAPIError(w http.ResponseWriter, err *APIError) {
	h.writeJSON(w, err.HTTPStatus, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    err.Code,
			Message: err.Message,
		},
	})
}

// HandleError maps a domain error to an API error and writes the response.
// Returns true if an error was handled, false if err was nil.
func (h *Handler) HandleError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	apiErr := MapDomainError(err)
	h.WriteAPIError(w, apiErr)
	return true
}

// HandleStoreError handles storage errors with logging.
// Returns true if an error was handled, false if err was nil.
func (h *Handler) HandleStoreError(w http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}

	// Check for known domain errors first
	apiErr := MapDomainError(err)
	
	// Log unexpected errors
	if apiErr.Code == ErrCodeInternalError {
		h.logger.Error().Err(err).Str("operation", operation).Msg("Storage operation failed")
		apiErr = &APIError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       ErrCodeStoreError,
			Message:    "Failed to " + operation,
		}
	}

	h.WriteAPIError(w, apiErr)
	return true
}
