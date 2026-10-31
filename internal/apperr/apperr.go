// Package apperr defines the errors modules return for expected failures:
// invalid input, missing or forbidden resources, conflicts. The HTTP layer
// turns them into RFC 9457 problem details (docs/api.md#conventions); any
// other error is an internal one, logged and answered with a bare 500.
//
// Codes are stable snake_case strings the SPA translates (error.<code>).
package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Error is an expected failure with an HTTP status and a stable code.
type Error struct {
	Status int
	Code   string
	Detail string  // English, for API users and logs; the SPA shows Code
	Fields []Field // per-field problems for Status 422
	// RetryAfter is set on 429 responses.
	RetryAfter time.Duration
}

// Field is one invalid input field.
type Field struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("%d %s", e.Status, e.Code)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	for _, f := range e.Fields {
		msg += fmt.Sprintf(" [%s: %s]", f.Field, f.Code)
	}
	return msg
}

// As returns the *Error in err's chain, if any.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Is reports whether err is an *Error with code.
func Is(err error, code string) bool {
	e, ok := As(err)
	return ok && e.Code == code
}

// Invalid is a 422 listing invalid fields. Add fields with Fields.Add and
// return Fields.Err.
func Invalid(fields ...Field) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_input", Fields: fields}
}

// Fields collects field problems while validating input.
type Fields []Field

// Add records that field failed with code.
func (f *Fields) Add(field, code string) {
	*f = append(*f, Field{Field: field, Code: code})
}

// Err is nil when no field failed, else an Invalid error.
func (f Fields) Err() error {
	if len(f) == 0 {
		return nil
	}
	return Invalid(f...)
}

// BadRequest is a 400 for malformed requests (bad JSON, wrong types).
func BadRequest(detail string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "bad_request", Detail: detail}
}

// Unauthorized is a 401: no valid credentials.
func Unauthorized(code string) *Error {
	return &Error{Status: http.StatusUnauthorized, Code: code}
}

// Forbidden is a 403: authenticated, but not allowed.
func Forbidden(code string) *Error {
	return &Error{Status: http.StatusForbidden, Code: code}
}

// NotFound is a 404.
func NotFound(code string) *Error {
	return &Error{Status: http.StatusNotFound, Code: code}
}

// Conflict is a 409.
func Conflict(code string) *Error {
	return &Error{Status: http.StatusConflict, Code: code}
}

// TooManyRequests is a 429; retry tells the client when to try again.
func TooManyRequests(code string, retry time.Duration) *Error {
	return &Error{Status: http.StatusTooManyRequests, Code: code, RetryAfter: retry}
}
