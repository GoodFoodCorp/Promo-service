package domain

import "fmt"

// Typed business errors, mapped to HTTP codes only in the adapter layer.
type ErrorCode string

const (
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound   ErrorCode = "NOT_FOUND"
	ErrCodeForbidden  ErrorCode = "FORBIDDEN"
	ErrCodeConflict   ErrorCode = "CONFLICT"
)

type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func NewValidationError(msg string) *Error { return &Error{Code: ErrCodeValidation, Message: msg} }
func NewNotFoundError(msg string) *Error   { return &Error{Code: ErrCodeNotFound, Message: msg} }
func NewForbiddenError(msg string) *Error  { return &Error{Code: ErrCodeForbidden, Message: msg} }
func NewConflictError(msg string) *Error   { return &Error{Code: ErrCodeConflict, Message: msg} }
