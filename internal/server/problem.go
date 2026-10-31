package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lidisoft/horizon/internal/apperr"
)

// Problem is an RFC 9457 problem details body (docs/api.md#conventions).
// Type is a stable error code the SPA translates; "about:blank" means the
// HTTP status says it all.
type Problem struct {
	Type   string         `json:"type" example:"about:blank"`
	Title  string         `json:"title" example:"Not Found"`
	Status int            `json:"status" example:"404"`
	Detail string         `json:"detail,omitempty"`
	Errors []apperr.Field `json:"errors,omitempty"`
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

// WriteError answers err: an *apperr.Error becomes its problem details, any
// other error is logged and answered with a bare 500 that leaks nothing.
func WriteError(c *gin.Context, err error) {
	e, ok := apperr.As(err)
	if !ok {
		log := Log(c)
		log.Error().Err(err).Str("route", c.FullPath()).Msg("request failed")
		WriteProblem(c, Problem{Status: http.StatusInternalServerError})
		return
	}
	if e.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(e.RetryAfter.Seconds()))))
	}
	WriteProblem(c, Problem{Type: e.Code, Status: e.Status, Detail: e.Detail, Errors: e.Fields})
}

// maxBody bounds JSON request bodies.
const maxBody = 1 << 20

// BindJSON decodes the request body into v, rejecting unknown fields, extra
// data and bodies over 1 MiB with a 400.
func BindJSON(c *gin.Context, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			return apperr.BadRequest("body over 1 MiB")
		case errors.Is(err, io.EOF):
			return apperr.BadRequest("empty body")
		}
		return apperr.BadRequest(fmt.Sprintf("invalid JSON: %v", err))
	}
	if dec.More() {
		return apperr.BadRequest("invalid JSON: data after the value")
	}
	return nil
}
