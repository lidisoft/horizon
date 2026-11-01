package report

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves /w/:ws/reports.
type HTTP struct {
	svc     *Service
	reports map[string]report
}

// report is a report's function, its result left untyped.
type report func(context.Context, workspace.Context, Request) (any, error)

func adapt[T any](fn func(context.Context, workspace.Context, Request) (T, error)) report {
	return func(ctx context.Context, wc workspace.Context, r Request) (any, error) {
		return fn(ctx, wc, r)
	}
}

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP {
	h := &HTTP{svc: svc}
	h.reports = map[string]report{
		"burndown":         adapt(svc.Burndown),
		"burnup":           adapt(svc.Burndown),
		"sprint":           adapt(svc.Sprint),
		"velocity":         adapt(svc.Velocity),
		"progress":         adapt(svc.Progress),
		"cfd":              adapt(svc.CumulativeFlow),
		"cycle-time":       adapt(svc.CycleTime),
		"throughput":       adapt(svc.Throughput),
		"distribution":     adapt(svc.Distribution),
		"created-resolved": adapt(svc.CreatedResolved),
	}
	return h
}

// Register mounts the routes on the workspace group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	ws.GET("/reports/:report", h.get)
	ws.POST("/reports/:report", h.post)
}

// requestFromQuery reads a Request from the URL (the shareable link):
// filter is JSON, lists are comma-separated.
func requestFromQuery(c *gin.Context) (Request, error) {
	var r Request
	var fields apperr.Fields
	if f := c.Query("filter"); f != "" {
		r.Filter = &query.Node{}
		if json.Unmarshal([]byte(f), r.Filter) != nil {
			fields.Add("filter", "invalid")
		}
	}
	r.Archived = c.Query("archived") == "true"
	if p := c.Query("projects"); p != "" {
		r.Projects = strings.Split(p, ",")
	}
	for name, dst := range map[string]*uuid.NullUUID{"iteration": &r.Iteration, "release": &r.Release, "milestone": &r.Milestone} {
		if v := c.Query(name); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				fields.Add(name, "invalid")
			}
			*dst = uuid.NullUUID{UUID: id, Valid: err == nil}
		}
	}
	r.Metric, r.From, r.To, r.Group, r.Period, r.By = c.Query("metric"), c.Query("from"), c.Query("to"), c.Query("group"), c.Query("period"), c.Query("by")
	if v := c.Query("last"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fields.Add("last", "invalid")
		}
		r.Last = n
	}
	return r, fields.Err()
}

// get runs a report from its URL.
//
//	@Summary		Run a report
//	@Description	report: burndown, burnup (SprintSeries; iteration=), sprint (SprintReport; iteration=), velocity (Velocity; projects=KEY, last=), progress (Progress; release= or milestone=), cfd (Flow; by=outcome|category), cycle-time (CycleTime), throughput and created-resolved (Periodic; period=week|month), distribution (Distribution; group=assignee|type|priority|label|project|cf:<id>); the last five over filter and projects. Options as query parameters, filter as JSON, lists comma-separated; metric=count|estimate (estimate falls back to count when scales are none or differ, and says so).
//	@Tags			reports
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			report		path		string	true	"Report"	Enums(burndown, burnup, sprint, velocity, progress, cfd, cycle-time, throughput, distribution, created-resolved)
//	@Param			filter		query		string	false	"Filter (JSON), as in /issues/query"
//	@Param			projects	query		string	false	"Project keys, comma-separated"
//	@Param			iteration	query		string	false	"Iteration id"
//	@Param			release		query		string	false	"Release id"
//	@Param			milestone	query		string	false	"Milestone id"
//	@Param			metric		query		string	false	"count or estimate"
//	@Param			from		query		string	false	"First day"
//	@Param			to			query		string	false	"Last day"
//	@Param			last		query		int		false	"Iterations (velocity)"
//	@Success		200			{object}	SprintSeries
//	@Router			/api/v1/w/{ws}/reports/{report} [get]
func (h *HTTP) get(c *gin.Context) {
	r, err := requestFromQuery(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	h.run(c, r)
}

// post runs a report from a JSON body.
//
//	@Summary		Run a report (JSON)
//	@Description	As GET, with the options in a JSON body.
//	@Tags			reports
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			report	path		string	true	"Report"	Enums(burndown, burnup, sprint, velocity, progress, cfd, cycle-time, throughput, distribution, created-resolved)
//	@Param			body	body		Request	true	"Scope and options"
//	@Success		200		{object}	SprintSeries
//	@Router			/api/v1/w/{ws}/reports/{report} [post]
func (h *HTTP) post(c *gin.Context) {
	var r Request
	if err := server.BindJSON(c, &r); err != nil {
		server.WriteError(c, err)
		return
	}
	h.run(c, r)
}

func (h *HTTP) run(c *gin.Context, r Request) {
	fn, ok := h.reports[c.Param("report")]
	if !ok {
		server.WriteError(c, apperr.NotFound("report_not_found"))
		return
	}
	out, err := fn(c, workspace.Current(c), r)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
