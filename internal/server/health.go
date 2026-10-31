package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// readyTimeout bounds the whole readiness probe.
const readyTimeout = 2 * time.Second

// Check is one readiness dependency.
type Check struct {
	Name  string
	Check func(ctx context.Context) error
}

// HealthResponse is the body of /healthz.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// ReadyResponse is the body of /readyz.
type ReadyResponse struct {
	Status string            `json:"status" example:"ready"`
	Checks map[string]string `json:"checks"`
}

func registerHealthRoutes(r *gin.Engine, checks []Check) {
	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(checks))
}

// healthz reports that the process is alive.
//
//	@Summary	Liveness probe
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	HealthResponse
//	@Router		/healthz [get]
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// readyz runs every check concurrently.
//
//	@Summary		Readiness probe
//	@Description	PostgreSQL, NATS and S3 reachable, schema at the expected version.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	ReadyResponse
//	@Failure		503	{object}	ReadyResponse
//	@Router			/readyz [get]
func readyz(checks []Check) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
		defer cancel()

		res := ReadyResponse{Status: "ready", Checks: make(map[string]string, len(checks))}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, chk := range checks {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result := "ok"
				if err := chk.Check(ctx); err != nil {
					result = err.Error()
				}
				mu.Lock()
				res.Checks[chk.Name] = result
				if result != "ok" {
					res.Status = "not ready"
				}
				mu.Unlock()
			}()
		}
		wg.Wait()

		status := http.StatusOK
		if res.Status != "ready" {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, res)
	}
}
