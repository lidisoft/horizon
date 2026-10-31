// Package server builds the Gin engine and runs it with graceful shutdown.
//
//	@title			Horizon API
//	@version		0.1.0
//	@description	Horizon project management API.
//	@basePath		/
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/lidisoft/horizon/internal/docs" // generated OpenAPI spec
	"github.com/lidisoft/horizon/internal/platform/metrics"
)

// ShutdownTimeout bounds how long in-flight requests may run after SIGTERM
// (docs/operations.md#shutdown).
const ShutdownTimeout = 30 * time.Second

// Deps groups what the HTTP server needs.
type Deps struct {
	Log     zerolog.Logger
	Checks  []Check
	Metrics *metrics.Registry
	Web     *Web // nil serves no web app (tests of the API alone)
}

// NewEngine creates the Gin engine with middleware and routes.
func NewEngine(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.ContextWithFallback = true

	r.HandleMethodNotAllowed = true
	r.Use(SecurityHeaders())
	r.Use(RequestID())
	r.Use(Logger(deps.Log))
	r.Use(Recovery())
	if deps.Metrics != nil {
		r.Use(Instrument(newHTTPMetrics(deps.Metrics)))
	}

	registerHealthRoutes(r, deps.Checks)

	swagger := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.DefaultModelsExpandDepth(-1))
	r.GET("/swagger/*any", func(c *gin.Context) {
		if c.Param("any") == "/" {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
			return
		}
		swagger(c)
	})

	if deps.Web != nil {
		deps.Web.register(r)
	}
	r.NoRoute(func(c *gin.Context) {
		if deps.Web != nil && isAppRoute(c.Request) {
			deps.Web.serveIndex(c)
			return
		}
		WriteProblem(c, Problem{Status: http.StatusNotFound})
	})
	r.NoMethod(func(c *gin.Context) {
		WriteProblem(c, Problem{Status: http.StatusMethodNotAllowed})
	})

	return r
}

// Run serves handler on addr until ctx is cancelled, then shuts down
// gracefully within ShutdownTimeout.
func Run(ctx context.Context, addr string, handler http.Handler, log zerolog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Msg("listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen %s: %w", addr, err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	log.Info().Str("addr", addr).Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown %s: %w", addr, err)
	}
	return <-errCh
}
