package server

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/logging"
)

// HeaderRequestID carries the request id in and out.
const HeaderRequestID = "X-Request-Id"

// validRequestID accepts ids from proxies, within reason.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// loggerKey stores the request-scoped logger in the Gin context.
const loggerKey = "horizon.logger"

// RequestID reuses a well-formed incoming X-Request-Id or generates a UUIDv7,
// and echoes it in the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			id = uuid.Must(uuid.NewV7()).String()
		}
		c.Set(HeaderRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// Logger stores a request-scoped logger (with the request id) in the context
// and logs each request when it completes.
func Logger(base zerolog.Logger) gin.HandlerFunc {
	httpLog := logging.WithSystem(base, "http")
	return func(c *gin.Context) {
		start := time.Now()
		log := httpLog.With().Str("request_id", c.GetString(HeaderRequestID)).Logger()
		c.Set(loggerKey, log)

		c.Next()

		status := c.Writer.Status()
		ev := log.Info()
		if status >= http.StatusInternalServerError {
			ev = log.Error()
		}
		ev.Str("method", c.Request.Method).
			Str("route", c.FullPath()).
			Str("path", c.Request.URL.Path).
			Int("status", status).
			Dur("latency", time.Since(start)).
			Str("ip", c.ClientIP()).
			Send()
	}
}

// Log returns the request-scoped logger set by Logger.
func Log(c *gin.Context) zerolog.Logger {
	if v, ok := c.Get(loggerKey); ok {
		return v.(zerolog.Logger)
	}
	return zerolog.Nop()
}

// Recovery turns panics into 500s and logs them with the request id.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		log := Log(c)
		log.Error().Interface("panic", err).Msg("panic serving request")
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

// Instrument records request counts and durations by route.
func Instrument(m *httpMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		m.requests.WithLabelValues(c.Request.Method, route, status).Inc()
		m.duration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
