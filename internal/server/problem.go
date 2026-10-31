package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Problem is an RFC 9457 problem details body (docs/api.md#conventions).
// Type is a stable error code the SPA translates; "about:blank" means the
// HTTP status says it all.
type Problem struct {
	Type   string       `json:"type" example:"about:blank"`
	Title  string       `json:"title" example:"Not Found"`
	Status int          `json:"status" example:"404"`
	Detail string       `json:"detail,omitempty"`
	Errors []FieldError `json:"errors,omitempty"`
}

// FieldError points at one invalid input field.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// WriteProblem aborts the request with a problem details response.
func WriteProblem(c *gin.Context, p Problem) {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(p.Status, p)
}
