package issue

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

const contextKey = "horizon.issue"

// HTTP serves the issue endpoints.
type HTTP struct {
	svc      *Service
	projects *project.Service
}

// NewHTTP returns the handlers.
func NewHTTP(svc *Service, projects *project.Service) *HTTP {
	return &HTTP{svc: svc, projects: projects}
}

// RegisterWorkspace mounts /w/:ws/issues routes.
func (h *HTTP) RegisterWorkspace(ws *gin.RouterGroup) {
	ws.POST("/issues", h.create)
	ws.POST("/issues/query", h.query)
	g := ws.Group("/issues/:issue", h.enter)
	g.GET("", h.get)
	g.PATCH("", h.update)
	g.DELETE("", h.delete)
	g.GET("/move/preview", h.previewMove)
	g.GET("/comments", h.comments)
	g.POST("/comments", h.addComment)
	g.PATCH("/comments/:id", h.editComment)
	g.DELETE("/comments/:id", h.deleteComment)
	g.GET("/attachments", h.attachments)
	g.POST("/attachments", h.startUpload)
	g.POST("/attachments/:id/complete", h.completeUpload)
	g.GET("/attachments/:id/download", h.download)
	g.DELETE("/attachments/:id", h.deleteAttachment)
	g.GET("/activity", h.activity)
	g.POST("/move", h.move)
	g.GET("/transitions", h.transitions)
	g.POST("/transition", h.transition)
	g.POST("/assign", h.assign)
	g.POST("/estimate", h.estimate)
	g.POST("/release", h.release)
	g.POST("/prioritize", h.prioritize)
	g.POST("/rank", h.rank)
	g.POST("/archive", h.archive)
	g.POST("/unarchive", h.unarchive)
	g.GET("/children", h.children)
	g.GET("/links", h.links)
	g.POST("/links", h.addLink)
	g.DELETE("/links/:id", h.removeLink)
}

// RegisterProject mounts the project's issue list on /w/:ws/projects/:key.
func (h *HTTP) RegisterProject(p *gin.RouterGroup) {
	p.GET("/issues", h.list)
}

func (h *HTTP) enter(c *gin.Context) {
	ic, err := h.svc.Enter(c, workspace.Current(c), c.Param("issue"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.Set(contextKey, ic)
	access.SetScope(c, ic.Scope())
	c.Next()
}

func current(c *gin.Context) Context { return c.MustGet(contextKey).(Context) }

// reply answers an issue with its version as ETag.
func reply(c *gin.Context, status int, i Issue, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, i.Version)
	c.JSON(status, i)
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

// QueryRequest is the body of POST /w/{ws}/issues/query: a query
// (docs/api.md#querying-issues), the projects to look in (keys; none: every
// project the caller sees) and the cursor of the page.
type QueryRequest struct {
	query.Query
	Projects []string `json:"projects,omitempty" example:"HZN"`
	Cursor   string   `json:"cursor,omitempty"`
}

// query finds issues.
//
//	@Summary		Query issues
//	@Description	Conditions on built-in fields (project, type, status, status.category, status.outcome, priority, assignee, reporter, parent, iteration, release, labels, estimate, title, text, start_date, due_date, created, updated, resolved, started, archived) and custom fields (cf:<id>), combined with and, or, not; sorted; a page at a time. Placeholders: @me, @today, @current_iteration; relative dates such as -7d. Archived issues only with archived: true. 422 names the place a query goes wrong.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		QueryRequest	true	"Query"
//	@Success		200		{object}	QueryResult
//	@Failure		422		{object}	server.Problem
//	@Router			/api/v1/w/{ws}/issues/query [post]
func (h *HTTP) query(c *gin.Context) {
	var req QueryRequest
	if !bind(c, &req) {
		return
	}
	res, err := h.svc.Query(c, workspace.Current(c), req.Query, req.Projects, req.Cursor)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// CreateRequest is the body of POST /w/{ws}/issues.
type CreateRequest struct {
	Project     string    `json:"project" example:"HZN"`
	TypeID      uuid.UUID `json:"type_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	// DescriptionDoc is the description as a block document; it wins over
	// description (plain text, one paragraph per line).
	DescriptionDoc json.RawMessage            `json:"description_doc,omitempty" swaggertype:"object"`
	Priority       string                     `json:"priority,omitempty" enums:"none,urgent,high,medium,low"`
	AssigneeID     *uuid.UUID                 `json:"assignee_id,omitempty"`
	Estimate       *string                    `json:"estimate,omitempty"`
	Parent         string                     `json:"parent,omitempty" example:"HZN-12"`
	Labels         []uuid.UUID                `json:"labels,omitempty"`
	StartDate      string                     `json:"start_date,omitempty" example:"2026-11-02"`
	DueDate        string                     `json:"due_date,omitempty"`
	Fields         map[string]json.RawMessage `json:"fields,omitempty" swaggertype:"object"`
}

// create adds an issue.
//
//	@Summary		Create an issue
//	@Description	Needs issue.create on the project; in the initial status of the type's workflow, last in rank. Required custom fields of the type must be set.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		CreateRequest	true	"Project key, type and title at least"
//	@Success		201		{object}	Issue
//	@Failure		422		{object}	server.Problem
//	@Router			/api/v1/w/{ws}/issues [post]
func (h *HTTP) create(c *gin.Context) {
	var req CreateRequest
	if !bind(c, &req) {
		return
	}
	pc, err := h.projects.Enter(c, workspace.Current(c), req.Project)
	if err != nil {
		server.WriteError(c, apperr.Invalid(apperr.Field{Field: "project", Code: "unknown"}))
		return
	}
	in := NewIssue{TypeID: req.TypeID, Title: req.Title, Description: Text{Plain: req.Description, Doc: req.DescriptionDoc}, Priority: req.Priority, Estimate: req.Estimate,
		ParentKey: req.Parent, Labels: req.Labels, StartDate: req.StartDate, DueDate: req.DueDate, Fields: req.Fields}
	if req.AssigneeID != nil {
		in.AssigneeID = uuid.NullUUID{UUID: *req.AssigneeID, Valid: true}
	}
	i, err := h.svc.Create(c, pc, in)
	reply(c, http.StatusCreated, i, err)
}

// list returns a project's issues in rank order.
//
//	@Summary	List a project's issues
//	@Tags		issues
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		key			path		string	true	"Project key"
//	@Param		cursor		query		string	false	"next_cursor of the previous page"
//	@Param		limit		query		int		false	"Page size, at most 500"
//	@Param		archived	query		bool	false	"Include archived issues"
//	@Success	200			{object}	Page
//	@Router		/api/v1/w/{ws}/projects/{key}/issues [get]
func (h *HTTP) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	p, err := h.svc.List(c, project.Current(c), c.Query("cursor"), limit, c.Query("archived") == "true")
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// get returns an issue by key.
//
//	@Summary		Get an issue
//	@Description	A former project key resolves too; the response carries the current key.
//	@Tags			issues
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			issue	path		string	true	"Issue key"
//	@Success		200		{object}	Issue
//	@Failure		404		{object}	server.Problem	"issue_not_found"
//	@Router			/api/v1/w/{ws}/issues/{issue} [get]
func (h *HTTP) get(c *gin.Context) {
	i, err := h.svc.Get(c, current(c))
	reply(c, http.StatusOK, i, err)
}

// UpdateRequest changes the fields that are present.
type UpdateRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	// DescriptionDoc replaces the description with a block document; it
	// wins over description (plain text).
	DescriptionDoc json.RawMessage            `json:"description_doc,omitempty" swaggertype:"object"`
	Labels         *[]uuid.UUID               `json:"labels,omitempty"`
	StartDate      *string                    `json:"start_date,omitempty"`
	DueDate        *string                    `json:"due_date,omitempty"`
	Fields         map[string]json.RawMessage `json:"fields,omitempty" swaggertype:"object"`
	Parent         *string                    `json:"parent,omitempty" example:"HZN-12"`
	TypeID         *uuid.UUID                 `json:"type_id,omitempty"`
	StatusID       *uuid.UUID                 `json:"status_id,omitempty"`
}

// update edits an issue.
//
//	@Summary		Update an issue
//	@Description	Needs issue.edit. "" clears dates and the parent; null removes a custom field value. A type whose workflow differs needs status_id in it.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			issue		path		string			true	"Issue key"
//	@Param			If-Match	header		string			false	"Issue version"
//	@Param			body		body		UpdateRequest	true	"Fields to change"
//	@Success		200			{object}	Issue
//	@Failure		409			{object}	server.Problem	"version_conflict, issue_archived"
//	@Failure		422			{object}	server.Problem
//	@Router			/api/v1/w/{ws}/issues/{issue} [patch]
func (h *HTTP) update(c *gin.Context) {
	var req UpdateRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Update(c, current(c), v, Update{Title: req.Title, Description: Text{Plain: req.Description, Doc: req.DescriptionDoc}, Labels: req.Labels,
			StartDate: req.StartDate, DueDate: req.DueDate, Fields: req.Fields, ParentKey: req.Parent, TypeID: req.TypeID, StatusID: req.StatusID})
		reply(c, http.StatusOK, i, err)
	}
}

// TransitionList is the body of GET …/transitions.
type TransitionList struct {
	Items []AvailableTransition `json:"items"`
}

// transitions lists the transitions available from the issue's status.
//
//	@Summary	List available transitions
//	@Tags		issues
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	TransitionList
//	@Router		/api/v1/w/{ws}/issues/{issue}/transitions [get]
func (h *HTTP) transitions(c *gin.Context) {
	items, err := h.svc.Transitions(c, current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TransitionList{Items: items})
}

// TransitionRequest names the target status.
type TransitionRequest struct {
	StatusID uuid.UUID `json:"status_id"`
}

// transition changes the issue's status.
//
//	@Summary		Transition an issue
//	@Description	Through an allowed transition of the workflow: 409 transition_not_allowed, children_open; 403 transition_forbidden (roles); 422 transition_requires_fields lists missing fields.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string				true	"Workspace slug"
//	@Param			issue		path		string				true	"Issue key"
//	@Param			If-Match	header		string				false	"Issue version"
//	@Param			body		body		TransitionRequest	true	"Target status"
//	@Success		200			{object}	Issue
//	@Router			/api/v1/w/{ws}/issues/{issue}/transition [post]
func (h *HTTP) transition(c *gin.Context) {
	var req TransitionRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Transition(c, current(c), v, req.StatusID)
		reply(c, http.StatusOK, i, err)
	}
}

// AssignRequest names the assignee; null unassigns.
type AssignRequest struct {
	AssigneeID *uuid.UUID `json:"assignee_id"`
}

// assign sets the assignee.
//
//	@Summary	Assign an issue
//	@Tags		issues
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string			true	"Workspace slug"
//	@Param		issue		path		string			true	"Issue key"
//	@Param		If-Match	header		string			false	"Issue version"
//	@Param		body		body		AssignRequest	true	"Assignee, or null"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/assign [post]
func (h *HTTP) assign(c *gin.Context) {
	var req AssignRequest
	if !bind(c, &req) {
		return
	}
	id := uuid.Nil
	if req.AssigneeID != nil {
		id = *req.AssigneeID
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Assign(c, current(c), v, id)
		reply(c, http.StatusOK, i, err)
	}
}

// ReleaseRequest names the release; null clears it.
type ReleaseRequest struct {
	ReleaseID *uuid.UUID `json:"release_id"`
}

// release targets the issue to a release.
//
//	@Summary		Set an issue's release
//	@Description	Needs issue.edit. A release of the issue's project, not archived (422 release_id unknown or archived); null clears it.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			issue		path		string			true	"Issue key"
//	@Param			If-Match	header		string			false	"Issue version"
//	@Param			body		body		ReleaseRequest	true	"Release, or null"
//	@Success		200			{object}	Issue
//	@Router			/api/v1/w/{ws}/issues/{issue}/release [post]
func (h *HTTP) release(c *gin.Context) {
	var req ReleaseRequest
	if !bind(c, &req) {
		return
	}
	var id uuid.NullUUID
	if req.ReleaseID != nil {
		id = uuid.NullUUID{UUID: *req.ReleaseID, Valid: true}
	}
	if v, ok := version(c); ok {
		i, err := h.svc.SetRelease(c, current(c), v, id)
		reply(c, http.StatusOK, i, err)
	}
}

// EstimateRequest sets the estimate; "" clears it.
type EstimateRequest struct {
	Estimate string `json:"estimate" example:"5"`
}

// estimate sets the estimate.
//
//	@Summary	Estimate an issue
//	@Tags		issues
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string			true	"Workspace slug"
//	@Param		issue		path		string			true	"Issue key"
//	@Param		If-Match	header		string			false	"Issue version"
//	@Param		body		body		EstimateRequest	true	"A value of the project's scale"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/estimate [post]
func (h *HTTP) estimate(c *gin.Context) {
	var req EstimateRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Estimate(c, current(c), v, req.Estimate)
		reply(c, http.StatusOK, i, err)
	}
}

// PrioritizeRequest sets the priority.
type PrioritizeRequest struct {
	Priority string `json:"priority" enums:"none,urgent,high,medium,low"`
}

// prioritize sets the priority.
//
//	@Summary	Prioritize an issue
//	@Tags		issues
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string				true	"Workspace slug"
//	@Param		issue		path		string				true	"Issue key"
//	@Param		If-Match	header		string				false	"Issue version"
//	@Param		body		body		PrioritizeRequest	true	"Priority"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/prioritize [post]
func (h *HTTP) prioritize(c *gin.Context) {
	var req PrioritizeRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Prioritize(c, current(c), v, req.Priority)
		reply(c, http.StatusOK, i, err)
	}
}

// RankRequest places the issue after another of its project; "" puts it
// first.
type RankRequest struct {
	After string `json:"after" example:"HZN-41"`
}

// rank moves the issue in its project's order.
//
//	@Summary	Rank an issue
//	@Tags		issues
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string		true	"Workspace slug"
//	@Param		issue		path		string		true	"Issue key"
//	@Param		If-Match	header		string		false	"Issue version"
//	@Param		body		body		RankRequest	true	"The issue to come after"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/rank [post]
func (h *HTTP) rank(c *gin.Context) {
	var req RankRequest
	if !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		i, err := h.svc.Rank(c, current(c), v, req.After)
		reply(c, http.StatusOK, i, err)
	}
}

// archive makes an issue read-only.
//
//	@Summary	Archive an issue
//	@Tags		issues
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		issue		path		string	true	"Issue key"
//	@Param		If-Match	header		string	false	"Issue version"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/archive [post]
func (h *HTTP) archive(c *gin.Context) { h.setArchived(c, true) }

// unarchive restores an archived issue.
//
//	@Summary	Unarchive an issue
//	@Tags		issues
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		issue		path		string	true	"Issue key"
//	@Param		If-Match	header		string	false	"Issue version"
//	@Success	200			{object}	Issue
//	@Router		/api/v1/w/{ws}/issues/{issue}/unarchive [post]
func (h *HTTP) unarchive(c *gin.Context) { h.setArchived(c, false) }

func (h *HTTP) setArchived(c *gin.Context, archived bool) {
	if v, ok := version(c); ok {
		i, err := h.svc.SetArchived(c, current(c), v, archived)
		reply(c, http.StatusOK, i, err)
	}
}

// IssueList is a list of issues.
type IssueList struct {
	Items []Issue `json:"items"`
}

// children lists the issue's children the caller can see.
//
//	@Summary	List an issue's children
//	@Tags		issues
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	IssueList
//	@Router		/api/v1/w/{ws}/issues/{issue}/children [get]
func (h *HTTP) children(c *gin.Context) {
	items, err := h.svc.Children(c, current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, IssueList{Items: items})
}

// LinkList is the body of GET …/links.
type LinkList struct {
	Items []Link `json:"items"`
}

// links lists the issue's links.
//
//	@Summary	List an issue's links
//	@Tags		issues
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	LinkList
//	@Router		/api/v1/w/{ws}/issues/{issue}/links [get]
func (h *HTTP) links(c *gin.Context) {
	items, err := h.svc.Links(c, current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, LinkList{Items: items})
}

// LinkRequest links the issue to another.
type LinkRequest struct {
	Type    string `json:"type" enums:"relates_to,blocks,blocked_by,duplicates,duplicated_by,precedes,follows"`
	Target  string `json:"target" example:"HZN-7"`
	LagDays *int32 `json:"lag_days,omitempty"`
}

// addLink creates a link.
//
//	@Summary		Link two issues
//	@Description	Needs issue.link. lag_days only for precedes and follows.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			issue	path		string		true	"Issue key"
//	@Param			body	body		LinkRequest	true	"Type and target"
//	@Success		201		{object}	Link
//	@Failure		409		{object}	server.Problem	"link_exists"
//	@Router			/api/v1/w/{ws}/issues/{issue}/links [post]
func (h *HTTP) addLink(c *gin.Context) {
	var req LinkRequest
	if !bind(c, &req) {
		return
	}
	l, err := h.svc.AddLink(c, current(c), LinkInput{Type: req.Type, TargetKey: req.Target, LagDays: req.LagDays})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, l)
}

// removeLink deletes a link.
//
//	@Summary	Unlink two issues
//	@Tags		issues
//	@Param		ws		path	string	true	"Workspace slug"
//	@Param		issue	path	string	true	"Issue key"
//	@Param		id		path	string	true	"Link id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/issues/{issue}/links/{id} [delete]
func (h *HTTP) removeLink(c *gin.Context) {
	id, ok := paramID(c, "link_not_found")
	if !ok {
		return
	}
	if err := h.svc.RemoveLink(c, current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// delete hard-deletes an issue.
//
//	@Summary		Delete an issue
//	@Description	Needs issue.delete. Its content and history go; an IssueDeleted tombstone remains. children is required when it has any: reparent moves them to its parent, cascade deletes the subtree (403 unless the caller may delete every descendant).
//	@Tags			issues
//	@Param			ws			path	string	true	"Workspace slug"
//	@Param			issue		path	string	true	"Issue key"
//	@Param			children	query	string	false	"reparent or cascade"	Enums(reparent, cascade)
//	@Success		204
//	@Failure		422	{object}	server.Problem	"children_choice_required"
//	@Router			/api/v1/w/{ws}/issues/{issue} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if err := h.svc.Delete(c, current(c), c.Query("children")); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// previewMove shows what a move would map.
//
//	@Summary	Preview moving an issue
//	@Tags		issues
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		issue		path		string	true	"Issue key"
//	@Param		project		query		string	true	"Target project key"
//	@Param		children	query		string	false	"move (default) or leave"	Enums(move, leave)
//	@Success	200			{object}	MovePreview
//	@Router		/api/v1/w/{ws}/issues/{issue}/move/preview [get]
func (h *HTTP) previewMove(c *gin.Context) {
	p, err := h.svc.PreviewMove(c, current(c), c.Query("project"), c.Query("children"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// MoveMapping overrides the automatic matches, by source id.
type MoveMapping struct {
	Types    map[uuid.UUID]uuid.UUID  `json:"types,omitempty"`
	Statuses map[uuid.UUID]uuid.UUID  `json:"statuses,omitempty"`
	Fields   map[uuid.UUID]*uuid.UUID `json:"fields,omitempty"`
}

// MoveRequest is the body of POST …/move.
type MoveRequest struct {
	Project  string      `json:"project" example:"OPS"`
	Children string      `json:"children,omitempty" enums:"move,leave"`
	Mapping  MoveMapping `json:"mapping"`
}

// move moves an issue to another project.
//
//	@Summary		Move an issue to another project
//	@Description	Needs issue.move, and issue.create on the target. Types and statuses that match neither by id nor by name must be mapped (422 mapping_required lists them); a field mapped to null drops its values. The response carries the new key; the old one keeps resolving.
//	@Tags			issues
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			issue	path		string		true	"Issue key"
//	@Param			body	body		MoveRequest	true	"Target and mapping"
//	@Success		200		{object}	Issue
//	@Failure		422		{object}	server.Problem	"mapping_required"
//	@Router			/api/v1/w/{ws}/issues/{issue}/move [post]
func (h *HTTP) move(c *gin.Context) {
	var req MoveRequest
	if !bind(c, &req) {
		return
	}
	in := MoveInput{Project: req.Project, Children: req.Children, Types: req.Mapping.Types, Statuses: req.Mapping.Statuses,
		Fields: map[uuid.UUID]uuid.UUID{}}
	for from, to := range req.Mapping.Fields {
		in.Fields[from] = uuid.Nil
		if to != nil {
			in.Fields[from] = *to
		}
	}
	i, err := h.svc.Move(c, current(c), in)
	reply(c, http.StatusOK, i, err)
}

func paramID(c *gin.Context, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound(notFound))
		return id, false
	}
	return id, true
}

// comments lists an issue's comments.
//
//	@Summary	List an issue's comments
//	@Tags		comments
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Param		cursor	query		string	false	"next_cursor of the previous page"
//	@Param		limit	query		int		false	"Page size, at most 200"
//	@Success	200		{object}	CommentPage
//	@Router		/api/v1/w/{ws}/issues/{issue}/comments [get]
func (h *HTTP) comments(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	p, err := h.svc.Comments(c, current(c), c.Query("cursor"), limit)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// CommentRequest is the body of a comment: a block document, or plain
// text (one paragraph per line).
type CommentRequest struct {
	Body    *string         `json:"body,omitempty"`
	BodyDoc json.RawMessage `json:"body_doc,omitempty" swaggertype:"object"`
}

// addComment comments on an issue.
//
//	@Summary	Comment on an issue
//	@Tags		comments
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string			true	"Workspace slug"
//	@Param		issue	path		string			true	"Issue key"
//	@Param		body	body		CommentRequest	true	"Comment"
//	@Success	201		{object}	Comment
//	@Router		/api/v1/w/{ws}/issues/{issue}/comments [post]
func (h *HTTP) addComment(c *gin.Context) {
	var req CommentRequest
	if !bind(c, &req) {
		return
	}
	cm, err := h.svc.AddComment(c, current(c), Text{Plain: req.Body, Doc: req.BodyDoc})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, cm.Version)
	c.JSON(http.StatusCreated, cm)
}

// editComment changes the caller's own comment.
//
//	@Summary	Edit a comment
//	@Tags		comments
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string			true	"Workspace slug"
//	@Param		issue		path		string			true	"Issue key"
//	@Param		id			path		string			true	"Comment id"
//	@Param		If-Match	header		string			false	"Comment version"
//	@Param		body		body		CommentRequest	true	"New body"
//	@Success	200			{object}	Comment
//	@Failure	403			{object}	server.Problem	"not_author"
//	@Router		/api/v1/w/{ws}/issues/{issue}/comments/{id} [patch]
func (h *HTTP) editComment(c *gin.Context) {
	var req CommentRequest
	id, ok := paramID(c, "comment_not_found")
	if !ok || !bind(c, &req) {
		return
	}
	if v, ok := version(c); ok {
		cm, err := h.svc.EditComment(c, current(c), id, v, Text{Plain: req.Body, Doc: req.BodyDoc})
		if err != nil {
			server.WriteError(c, err)
			return
		}
		server.SetETag(c, cm.Version)
		c.JSON(http.StatusOK, cm)
	}
}

// deleteComment removes a comment.
//
//	@Summary		Delete a comment
//	@Description	The author's own, or any with issue.comment.moderate.
//	@Tags			comments
//	@Param			ws		path	string	true	"Workspace slug"
//	@Param			issue	path	string	true	"Issue key"
//	@Param			id		path	string	true	"Comment id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/issues/{issue}/comments/{id} [delete]
func (h *HTTP) deleteComment(c *gin.Context) {
	id, ok := paramID(c, "comment_not_found")
	if !ok {
		return
	}
	if err := h.svc.DeleteComment(c, current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AttachmentList is the body of GET …/attachments.
type AttachmentList struct {
	Items []Attachment `json:"items"`
}

// attachments lists an issue's attachments.
//
//	@Summary	List an issue's attachments
//	@Tags		attachments
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	AttachmentList
//	@Router		/api/v1/w/{ws}/issues/{issue}/attachments [get]
func (h *HTTP) attachments(c *gin.Context) {
	items, err := h.svc.Attachments(c, current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, AttachmentList{Items: items})
}

// UploadRequest announces a file to attach.
type UploadRequest struct {
	Filename    string `json:"filename" example:"trace.log"`
	ContentType string `json:"content_type,omitempty" example:"text/plain"`
	Size        int64  `json:"size" example:"2048"`
}

// startUpload starts an attachment upload.
//
//	@Summary		Start an attachment upload
//	@Description	Needs issue.attach. PUT the bytes to upload_url (presigned, 15 minutes), then POST …/complete. At most 100 MiB.
//	@Tags			attachments
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			issue	path		string			true	"Issue key"
//	@Param			body	body		UploadRequest	true	"The file"
//	@Success		201		{object}	Upload
//	@Router			/api/v1/w/{ws}/issues/{issue}/attachments [post]
func (h *HTTP) startUpload(c *gin.Context) {
	var req UploadRequest
	if !bind(c, &req) {
		return
	}
	u, err := h.svc.StartUpload(c, current(c), req.Filename, req.ContentType, req.Size)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

// completeUpload makes an uploaded attachment visible.
//
//	@Summary		Complete an attachment upload
//	@Description	The object must be stored, with the announced size (409 upload_missing, 422 size: mismatch). The content type is sniffed.
//	@Tags			attachments
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			issue	path		string	true	"Issue key"
//	@Param			id		path		string	true	"Attachment id"
//	@Success		200		{object}	Attachment
//	@Router			/api/v1/w/{ws}/issues/{issue}/attachments/{id}/complete [post]
func (h *HTTP) completeUpload(c *gin.Context) {
	id, ok := paramID(c, "attachment_not_found")
	if !ok {
		return
	}
	a, err := h.svc.CompleteUpload(c, current(c), id)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

// download redirects to a short-lived download URL.
//
//	@Summary		Download an attachment
//	@Description	Redirects to a presigned URL (5 minutes). PNG, JPEG, GIF and WebP open inline; everything else downloads.
//	@Tags			attachments
//	@Param			ws		path	string	true	"Workspace slug"
//	@Param			issue	path	string	true	"Issue key"
//	@Param			id		path	string	true	"Attachment id"
//	@Success		302
//	@Router			/api/v1/w/{ws}/issues/{issue}/attachments/{id}/download [get]
func (h *HTTP) download(c *gin.Context) {
	id, ok := paramID(c, "attachment_not_found")
	if !ok {
		return
	}
	u, err := h.svc.DownloadURL(c, current(c), id)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Redirect(http.StatusFound, u)
}

// deleteAttachment removes an attachment.
//
//	@Summary		Delete an attachment
//	@Description	The uploader's own (issue.attach), or any with issue.comment.moderate. The object is purged in the background.
//	@Tags			attachments
//	@Param			ws		path	string	true	"Workspace slug"
//	@Param			issue	path	string	true	"Issue key"
//	@Param			id		path	string	true	"Attachment id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/issues/{issue}/attachments/{id} [delete]
func (h *HTTP) deleteAttachment(c *gin.Context) {
	id, ok := paramID(c, "attachment_not_found")
	if !ok {
		return
	}
	if err := h.svc.DeleteAttachment(c, current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// activity returns an issue's history.
//
//	@Summary		An issue's activity
//	@Description	Its domain events, newest first, in the public event format: ids of statuses, types, labels, fields, people and visible issues are named.
//	@Tags			issues
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			issue	path		string	true	"Issue key"
//	@Param			cursor	query		string	false	"next_cursor of the previous page"
//	@Param			limit	query		int		false	"Page size, at most 100"
//	@Success		200		{object}	ActivityPage
//	@Router			/api/v1/w/{ws}/issues/{issue}/activity [get]
func (h *HTTP) activity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	p, err := h.svc.Activity(c, current(c), c.Query("cursor"), limit)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}
