package planning

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/server"
)

// HTTP serves the planning endpoints under /w/:ws/projects/:key.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// RegisterProject mounts the routes on the project group.
func (h *HTTP) RegisterProject(p *gin.RouterGroup) {
	p.GET("/iterations", h.iterations)
	p.POST("/iterations", h.plan)
	g := p.Group("/iterations/:id")
	g.GET("", h.iteration)
	g.PATCH("", h.update)
	g.DELETE("", h.delete)
	g.POST("/issues", h.load)
	g.POST("/start", h.start)
	g.POST("/complete", h.complete)
	g.POST("/archive", h.archive)
}

func reply(c *gin.Context, status int, it Iteration, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, it.Version)
	c.JSON(status, it)
}

func bind(c *gin.Context, v any) bool {
	if err := server.BindJSON(c, v); err != nil {
		server.WriteError(c, err)
		return false
	}
	return true
}

func version(c *gin.Context) (int32, bool) {
	v, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return 0, false
	}
	return v, true
}

func idParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("iteration_not_found"))
		return uuid.Nil, false
	}
	return id, true
}

// iterations lists the project's iterations.
//
//	@Summary		List iterations
//	@Description	Active first, then upcoming by start date, then finished ones (archived ones with ?archived=true). Each carries stats of the issues it holds; the list adds the suggested next iteration, the iteration settings in effect and the velocity of the last three completed ones.
//	@Tags			planning
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			key			path		string	true	"Project key"
//	@Param			archived	query		bool	false	"Include archived iterations"
//	@Success		200			{object}	IterationList
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations [get]
func (h *HTTP) iterations(c *gin.Context) {
	list, err := h.svc.Iterations(c, project.Current(c), c.Query("archived") == "true")
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// PlanRequest is the body of POST …/iterations.
type PlanRequest struct {
	Name      string `json:"name" example:"Sprint 5"`
	Goal      string `json:"goal,omitempty"`
	StartDate string `json:"start_date,omitempty" example:"2026-11-16"`
	EndDate   string `json:"end_date,omitempty" example:"2026-11-27"`
}

// plan adds an iteration.
//
//	@Summary		Plan an iteration
//	@Description	Needs iteration.manage. With both dates it is scheduled at once; without, planned.
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			key		path		string		true	"Project key"
//	@Param			body	body		PlanRequest	true	"Name required"
//	@Success		201		{object}	Iteration
//	@Failure		422		{object}	server.Problem
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations [post]
func (h *HTTP) plan(c *gin.Context) {
	var req PlanRequest
	if bind(c, &req) {
		it, err := h.svc.Plan(c, project.Current(c), PlanInput(req))
		reply(c, http.StatusCreated, it, err)
	}
}

// iteration returns one iteration.
//
//	@Summary	Get an iteration
//	@Tags		planning
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key"
//	@Param		id	path		string	true	"Iteration id"
//	@Success	200	{object}	Iteration
//	@Router		/api/v1/w/{ws}/projects/{key}/iterations/{id} [get]
func (h *HTTP) iteration(c *gin.Context) {
	if id, ok := idParam(c); ok {
		it, err := h.svc.Iteration(c, project.Current(c), id)
		reply(c, http.StatusOK, it, err)
	}
}

// UpdateRequest is the body of PATCH …/iterations/{id}: fields left out
// are unchanged.
type UpdateRequest struct {
	Name *string `json:"name,omitempty"`
	Goal *string `json:"goal,omitempty"`
	// StartDate and EndDate "" unschedule a planned or scheduled iteration.
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
}

// update changes an iteration.
//
//	@Summary		Update an iteration
//	@Description	Name and goal (IterationUpdated), dates (IterationScheduled). An active iteration keeps its start date; completed and archived ones don't change (409 iteration_closed).
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			key			path		string			true	"Project key"
//	@Param			id			path		string			true	"Iteration id"
//	@Param			If-Match	header		string			false	"Iteration version"
//	@Param			body		body		UpdateRequest	true	"Changes"
//	@Success		200			{object}	Iteration
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations/{id} [patch]
func (h *HTTP) update(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req UpdateRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		it, err := h.svc.Update(c, project.Current(c), id, v, UpdateInput(req))
		reply(c, http.StatusOK, it, err)
	}
}

// LoadRequest is the body of POST …/iterations/{id}/issues.
type LoadRequest struct {
	Add    []string `json:"add,omitempty" example:"HZN-41,HZN-44"`
	Remove []string `json:"remove,omitempty"`
}

// load moves issues into and out of an iteration.
//
//	@Summary		Load an iteration
//	@Description	Needs iteration.manage. Adds issues of the project (taking them from another iteration if they are in one) and removes some; at most 500 at once. Completed iterations don't change (409 iteration_closed).
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			key		path		string		true	"Project key"
//	@Param			id		path		string		true	"Iteration id"
//	@Param			body	body		LoadRequest	true	"Issue keys"
//	@Success		200		{object}	Iteration
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations/{id}/issues [post]
func (h *HTTP) load(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req LoadRequest
	if bind(c, &req) {
		it, err := h.svc.Load(c, project.Current(c), id, LoadInput(req))
		reply(c, http.StatusOK, it, err)
	}
}

// start makes a scheduled iteration active.
//
//	@Summary		Start an iteration
//	@Description	Needs iteration.manage and dates (409 iteration_not_scheduled), within the project's maximum of active iterations (409 too_many_active).
//	@Tags			planning
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			key			path		string	true	"Project key"
//	@Param			id			path		string	true	"Iteration id"
//	@Param			If-Match	header		string	false	"Iteration version"
//	@Success		200			{object}	Iteration
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations/{id}/start [post]
func (h *HTTP) start(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if v, ok := version(c); ok {
		it, err := h.svc.Start(c, project.Current(c), id, v)
		reply(c, http.StatusOK, it, err)
	}
}

// CompleteRequest is the body of POST …/iterations/{id}/complete.
type CompleteRequest struct {
	// RolloverTo is the open iteration taking the unfinished issues under
	// archive_done_rollover_rest; null or absent sends them to the backlog.
	RolloverTo *uuid.UUID `json:"rollover_to,omitempty"`
}

// complete closes an active iteration.
//
//	@Summary		Complete an iteration
//	@Description	Needs iteration.manage. The completion policy in effect is recorded and applied to the issues shortly after, by the policy worker: archive_all archives them all; archive_done_rollover_rest archives the finished ones and moves the rest to rollover_to or the backlog; none changes nothing.
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			key			path		string			true	"Project key"
//	@Param			id			path		string			true	"Iteration id"
//	@Param			If-Match	header		string			false	"Iteration version"
//	@Param			body		body		CompleteRequest	false	"Where unfinished issues go"
//	@Success		200			{object}	Iteration
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations/{id}/complete [post]
func (h *HTTP) complete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req CompleteRequest
	if c.Request.ContentLength != 0 && !bind(c, &req) {
		return
	}
	var to uuid.NullUUID
	if req.RolloverTo != nil {
		to = uuid.NullUUID{UUID: *req.RolloverTo, Valid: true}
	}
	if v, ok := version(c); ok {
		it, err := h.svc.Complete(c, project.Current(c), id, v, to)
		reply(c, http.StatusOK, it, err)
	}
}

// archive hides a completed iteration from planning.
//
//	@Summary	Archive an iteration
//	@Tags		planning
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		key			path		string	true	"Project key"
//	@Param		id			path		string	true	"Iteration id"
//	@Param		If-Match	header		string	false	"Iteration version"
//	@Success	200			{object}	Iteration
//	@Router		/api/v1/w/{ws}/projects/{key}/iterations/{id}/archive [post]
func (h *HTTP) archive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if v, ok := version(c); ok {
		it, err := h.svc.Archive(c, project.Current(c), id, v)
		reply(c, http.StatusOK, it, err)
	}
}

// delete hard-deletes an iteration.
//
//	@Summary		Delete an iteration
//	@Description	Needs iteration.manage; not while active (409 iteration_active). Its issues go back to the backlog; only an IterationDeleted tombstone remains of its history.
//	@Tags			planning
//	@Param			ws	path	string	true	"Workspace slug"
//	@Param			key	path	string	true	"Project key"
//	@Param			id	path	string	true	"Iteration id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/projects/{key}/iterations/{id} [delete]
func (h *HTTP) delete(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c, project.Current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
