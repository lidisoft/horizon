package planning

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
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
	p.GET("/releases", h.releases)
	p.POST("/releases", h.createRelease)
	r := p.Group("/releases/:id")
	r.GET("", h.release)
	r.PATCH("", h.updateRelease)
	r.DELETE("", h.deleteRelease)
	r.POST("/release", h.shipRelease)
	r.POST("/archive", h.archiveRelease)
	p.GET("/milestones", h.projectMilestones)
}

// RegisterWorkspace mounts the workspace's milestones on /w/:ws.
func (h *HTTP) RegisterWorkspace(ws *gin.RouterGroup) {
	ws.GET("/milestones", h.milestones)
	ws.POST("/milestones", h.createMilestone)
	m := ws.Group("/milestones/:id")
	m.GET("", h.milestone)
	m.PATCH("", h.updateMilestone)
	m.DELETE("", h.deleteMilestone)
	m.GET("/issues", h.milestoneIssues)
	m.POST("/issues", h.linkMilestone)
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

func idParam(c *gin.Context) (uuid.UUID, bool) { return idOf(c, "iteration_not_found") }

func idOf(c *gin.Context, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound(notFound))
		return uuid.Nil, false
	}
	return id, true
}

func send(c *gin.Context, status int, v any, version int32, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	if version > 0 {
		server.SetETag(c, version)
	}
	c.JSON(status, v)
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

// --- Releases ---------------------------------------------------------------------

// releases lists the project's releases.
//
//	@Summary		List releases
//	@Description	Unreleased ones by release date, then released ones, latest first (archived ones with ?archived=true), each with the progress of its issues.
//	@Tags			planning
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			key			path		string	true	"Project key"
//	@Param			archived	query		bool	false	"Include archived releases"
//	@Success		200			{object}	ReleaseList
//	@Router			/api/v1/w/{ws}/projects/{key}/releases [get]
func (h *HTTP) releases(c *gin.Context) {
	list, err := h.svc.Releases(c, project.Current(c), c.Query("archived") == "true")
	send(c, http.StatusOK, list, 0, err)
}

// ReleaseRequest creates a release or changes one (fields left out are
// unchanged; "" clears a date).
type ReleaseRequest struct {
	Name        *string `json:"name,omitempty" example:"v0.1"`
	Description *string `json:"description,omitempty"`
	StartDate   *string `json:"start_date,omitempty" example:"2026-11-02"`
	ReleaseDate *string `json:"release_date,omitempty" example:"2026-12-18"`
}

// createRelease adds a release.
//
//	@Summary		Create a release
//	@Description	Needs release.manage. Names are unique in the project, ignoring case.
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			key		path		string			true	"Project key"
//	@Param			body	body		ReleaseRequest	true	"Name required"
//	@Success		201		{object}	Release
//	@Router			/api/v1/w/{ws}/projects/{key}/releases [post]
func (h *HTTP) createRelease(c *gin.Context) {
	var req ReleaseRequest
	if bind(c, &req) {
		r, err := h.svc.CreateRelease(c, project.Current(c), ReleaseInput(req))
		send(c, http.StatusCreated, r, r.Version, err)
	}
}

// release returns one release.
//
//	@Summary	Get a release
//	@Tags		planning
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key"
//	@Param		id	path		string	true	"Release id"
//	@Success	200	{object}	Release
//	@Router		/api/v1/w/{ws}/projects/{key}/releases/{id} [get]
func (h *HTTP) release(c *gin.Context) {
	if id, ok := idOf(c, "release_not_found"); ok {
		r, err := h.svc.Release(c, project.Current(c), id)
		send(c, http.StatusOK, r, r.Version, err)
	}
}

// updateRelease changes a release.
//
//	@Summary		Update a release
//	@Description	Archived releases don't change (409 release_archived).
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			key			path		string			true	"Project key"
//	@Param			id			path		string			true	"Release id"
//	@Param			If-Match	header		string			false	"Release version"
//	@Param			body		body		ReleaseRequest	true	"Changes"
//	@Success		200			{object}	Release
//	@Router			/api/v1/w/{ws}/projects/{key}/releases/{id} [patch]
func (h *HTTP) updateRelease(c *gin.Context) {
	id, ok := idOf(c, "release_not_found")
	if !ok {
		return
	}
	var req ReleaseRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		r, err := h.svc.UpdateRelease(c, project.Current(c), id, v, ReleaseInput(req))
		send(c, http.StatusOK, r, r.Version, err)
	}
}

// shipRelease marks a release released.
//
//	@Summary		Release a release
//	@Description	Needs release.manage. The release date becomes today if it had none; issues stay targeted to it.
//	@Tags			planning
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			key			path		string	true	"Project key"
//	@Param			id			path		string	true	"Release id"
//	@Param			If-Match	header		string	false	"Release version"
//	@Success		200			{object}	Release
//	@Router			/api/v1/w/{ws}/projects/{key}/releases/{id}/release [post]
func (h *HTTP) shipRelease(c *gin.Context) {
	id, ok := idOf(c, "release_not_found")
	if !ok {
		return
	}
	if v, ok := version(c); ok {
		r, err := h.svc.ShipRelease(c, project.Current(c), id, v)
		send(c, http.StatusOK, r, r.Version, err)
	}
}

// archiveRelease hides a release.
//
//	@Summary	Archive a release
//	@Tags		planning
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		key			path		string	true	"Project key"
//	@Param		id			path		string	true	"Release id"
//	@Param		If-Match	header		string	false	"Release version"
//	@Success	200			{object}	Release
//	@Router		/api/v1/w/{ws}/projects/{key}/releases/{id}/archive [post]
func (h *HTTP) archiveRelease(c *gin.Context) {
	id, ok := idOf(c, "release_not_found")
	if !ok {
		return
	}
	if v, ok := version(c); ok {
		r, err := h.svc.ArchiveRelease(c, project.Current(c), id, v)
		send(c, http.StatusOK, r, r.Version, err)
	}
}

// deleteRelease hard-deletes a release.
//
//	@Summary		Delete a release
//	@Description	Needs release.manage. Its issues lose it; only a ReleaseDeleted tombstone remains of its history.
//	@Tags			planning
//	@Param			ws	path	string	true	"Workspace slug"
//	@Param			key	path	string	true	"Project key"
//	@Param			id	path	string	true	"Release id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/projects/{key}/releases/{id} [delete]
func (h *HTTP) deleteRelease(c *gin.Context) {
	if id, ok := idOf(c, "release_not_found"); ok {
		if err := h.svc.DeleteRelease(c, project.Current(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// --- Milestones -------------------------------------------------------------------

// milestones lists the milestones the caller sees.
//
//	@Summary		List milestones
//	@Description	The workspace's milestones and those of visible projects, by date, each with the progress of its linked issues.
//	@Tags			planning
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	MilestoneList
//	@Router			/api/v1/w/{ws}/milestones [get]
func (h *HTTP) milestones(c *gin.Context) {
	list, err := h.svc.Milestones(c, workspace.Current(c), "")
	send(c, http.StatusOK, list, 0, err)
}

// projectMilestones lists a project's milestones and the workspace's.
//
//	@Summary	List a project's milestones
//	@Tags		planning
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key"
//	@Success	200	{object}	MilestoneList
//	@Router		/api/v1/w/{ws}/projects/{key}/milestones [get]
func (h *HTTP) projectMilestones(c *gin.Context) {
	pc := project.Current(c)
	list, err := h.svc.Milestones(c, pc.Context, pc.Project.Key)
	send(c, http.StatusOK, list, 0, err)
}

// MilestoneRequest creates a milestone or changes one (fields left out are
// unchanged). Project applies on creation: a project key, or none for a
// workspace milestone.
type MilestoneRequest struct {
	Project     string  `json:"project,omitempty" example:"HZN"`
	Name        *string `json:"name,omitempty" example:"Beta"`
	Description *string `json:"description,omitempty"`
	Date        *string `json:"date,omitempty" example:"2026-12-18"`
}

// createMilestone adds a milestone.
//
//	@Summary		Create a milestone
//	@Description	Needs milestone.manage on the project, or on the workspace for a workspace milestone. Name and date required.
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string				true	"Workspace slug"
//	@Param			body	body		MilestoneRequest	true	"Milestone"
//	@Success		201		{object}	Milestone
//	@Router			/api/v1/w/{ws}/milestones [post]
func (h *HTTP) createMilestone(c *gin.Context) {
	var req MilestoneRequest
	if bind(c, &req) {
		m, err := h.svc.CreateMilestone(c, workspace.Current(c), MilestoneInput(req))
		send(c, http.StatusCreated, m, m.Version, err)
	}
}

// milestone returns one milestone.
//
//	@Summary	Get a milestone
//	@Tags		planning
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Milestone id"
//	@Success	200	{object}	Milestone
//	@Router		/api/v1/w/{ws}/milestones/{id} [get]
func (h *HTTP) milestone(c *gin.Context) {
	if id, ok := idOf(c, "milestone_not_found"); ok {
		m, err := h.svc.Milestone(c, workspace.Current(c), id)
		send(c, http.StatusOK, m, m.Version, err)
	}
}

// updateMilestone changes a milestone.
//
//	@Summary	Update a milestone
//	@Tags		planning
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string				true	"Workspace slug"
//	@Param		id			path		string				true	"Milestone id"
//	@Param		If-Match	header		string				false	"Milestone version"
//	@Param		body		body		MilestoneRequest	true	"Changes"
//	@Success	200			{object}	Milestone
//	@Router		/api/v1/w/{ws}/milestones/{id} [patch]
func (h *HTTP) updateMilestone(c *gin.Context) {
	id, ok := idOf(c, "milestone_not_found")
	if !ok {
		return
	}
	var req MilestoneRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		m, err := h.svc.UpdateMilestone(c, workspace.Current(c), id, v, MilestoneInput(req))
		send(c, http.StatusOK, m, m.Version, err)
	}
}

// MilestoneIssueList is the keys of a milestone's issues.
type MilestoneIssueList struct {
	Items []string `json:"items" example:"HZN-41,OPS-7"`
}

// milestoneIssues lists a milestone's linked issues.
//
//	@Summary		List a milestone's issues
//	@Description	The keys of the linked issues the caller sees.
//	@Tags			planning
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Param			id	path		string	true	"Milestone id"
//	@Success		200	{object}	MilestoneIssueList
//	@Router			/api/v1/w/{ws}/milestones/{id}/issues [get]
func (h *HTTP) milestoneIssues(c *gin.Context) {
	if id, ok := idOf(c, "milestone_not_found"); ok {
		keys, err := h.svc.MilestoneIssues(c, workspace.Current(c), id)
		send(c, http.StatusOK, MilestoneIssueList{Items: keys}, 0, err)
	}
}

// LinkRequest is the body of POST …/milestones/{id}/issues.
type LinkRequest struct {
	Add    []string `json:"add,omitempty" example:"HZN-41"`
	Remove []string `json:"remove,omitempty"`
}

// linkMilestone links and unlinks issues.
//
//	@Summary		Link issues to a milestone
//	@Description	Needs milestone.manage on its scope. A project milestone takes its project's issues only.
//	@Tags			planning
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			id		path		string		true	"Milestone id"
//	@Param			body	body		LinkRequest	true	"Issue keys"
//	@Success		200		{object}	Milestone
//	@Router			/api/v1/w/{ws}/milestones/{id}/issues [post]
func (h *HTTP) linkMilestone(c *gin.Context) {
	id, ok := idOf(c, "milestone_not_found")
	if !ok {
		return
	}
	var req LinkRequest
	if bind(c, &req) {
		m, err := h.svc.LinkMilestone(c, workspace.Current(c), id, LinkInput(req))
		send(c, http.StatusOK, m, m.Version, err)
	}
}

// deleteMilestone hard-deletes a milestone.
//
//	@Summary		Delete a milestone
//	@Description	Needs milestone.manage on its scope; only a MilestoneDeleted tombstone remains of its history.
//	@Tags			planning
//	@Param			ws	path	string	true	"Workspace slug"
//	@Param			id	path	string	true	"Milestone id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/milestones/{id} [delete]
func (h *HTTP) deleteMilestone(c *gin.Context) {
	if id, ok := idOf(c, "milestone_not_found"); ok {
		if err := h.svc.DeleteMilestone(c, workspace.Current(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
