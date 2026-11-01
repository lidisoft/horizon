package project

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

const contextKey = "horizon.project"

// HTTP serves the project endpoints under /w/:ws.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts the routes on the /w/:ws group; more registers other
// modules' routes under /projects/:key, after the project is resolved.
func (h *HTTP) Register(ws *gin.RouterGroup, more ...func(p *gin.RouterGroup)) {
	ws.GET("/projects", h.list)
	ws.POST("/projects", h.create)
	p := ws.Group("/projects/:key", h.enter)
	p.GET("", h.get)
	p.PATCH("", h.update)
	p.POST("/archive", h.archive)
	p.POST("/unarchive", h.unarchive)
	p.DELETE("", h.delete)
	for _, register := range more {
		register(p)
	}
}

// enter resolves /projects/:key (current or former key) and narrows the
// access scope to the project.
func (h *HTTP) enter(c *gin.Context) {
	pc, err := h.svc.Enter(c, workspace.Current(c), c.Param("key"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.Set(contextKey, pc)
	access.SetScope(c, pc.Scope())
	c.Next()
}

// Current returns the project resolved for the request.
func Current(c *gin.Context) Context { return c.MustGet(contextKey).(Context) }

// ProjectList is the body of GET /projects.
type ProjectList struct {
	Items []Project `json:"items"`
}

// list returns the projects the caller sees.
//
//	@Summary		List projects
//	@Description	Private projects appear to people with a grant on them and to holders of project.manage on the workspace.
//	@Tags			projects
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	ProjectList
//	@Router			/api/v1/w/{ws}/projects [get]
func (h *HTTP) list(c *gin.Context) {
	items, err := h.svc.List(c, workspace.Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ProjectList{Items: items})
}

// ProjectRequest is the body of POST and PATCH /projects.
type ProjectRequest struct {
	Key         *string    `json:"key,omitempty" example:"HZN"`
	Name        *string    `json:"name,omitempty" example:"Horizon"`
	Description *string    `json:"description,omitempty"`
	Colour      *string    `json:"colour,omitempty" enums:"blue,green,teal,purple,pink,coral,orange,yellow,grey"`
	LeadID      *uuid.UUID `json:"lead_id,omitempty"`
	Visibility  *string    `json:"visibility,omitempty" enums:"workspace,private"`
}

// create adds a project.
//
//	@Summary		Create a project
//	@Description	Needs project.create. The key is 2–10 upper-case letters or digits, starting with a letter, unused by any project now or before.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		ProjectRequest	true	"Key and name required"
//	@Success		201		{object}	Project
//	@Failure		422		{object}	server.Problem	"key invalid or taken"
//	@Router			/api/v1/w/{ws}/projects [post]
func (h *HTTP) create(c *gin.Context) {
	var req ProjectRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	p, err := h.svc.Create(c, workspace.Current(c), Input(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, p.Version)
	c.JSON(http.StatusCreated, p)
}

// ProjectResponse is a project with the caller's permissions on it.
type ProjectResponse struct {
	Project
	// Can lists the permissions the caller holds on the project.
	Can []access.Permission `json:"can"`
}

// get returns a project; a former key answers with the current one.
//
//	@Summary	Get a project
//	@Tags		projects
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key, current or former"
//	@Success	200	{object}	ProjectResponse
//	@Failure	404	{object}	server.Problem	"project_not_found"
//	@Router		/api/v1/w/{ws}/projects/{key} [get]
func (h *HTTP) get(c *gin.Context) {
	pc := Current(c)
	can, err := h.svc.access.Permissions(c, pc.Principal, pc.Scope())
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, pc.Project.Version)
	c.JSON(http.StatusOK, ProjectResponse{Project: pc.Project, Can: can})
}

// update edits a project.
//
//	@Summary		Update a project
//	@Description	Needs project.manage. A new key keeps the old one as an alias.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			key			path		string			true	"Project key"
//	@Param			If-Match	header		string			false	"ETag from GET"
//	@Param			body		body		ProjectRequest	true	"Fields to change; lead_id 00000000-0000-0000-0000-000000000000 clears the lead"
//	@Success		200			{object}	Project
//	@Failure		409			{object}	server.Problem	"version_conflict"
//	@Failure		422			{object}	server.Problem
//	@Router			/api/v1/w/{ws}/projects/{key} [patch]
func (h *HTTP) update(c *gin.Context) {
	var req ProjectRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	p, err := h.svc.Update(c, Current(c), version, Input(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, p.Version)
	c.JSON(http.StatusOK, p)
}

// archive makes a project read-only and hides it from the sidebar.
//
//	@Summary	Archive a project
//	@Tags		projects
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key"
//	@Success	200	{object}	Project
//	@Router		/api/v1/w/{ws}/projects/{key}/archive [post]
func (h *HTTP) archive(c *gin.Context) { h.setArchived(c, true) }

// unarchive restores an archived project.
//
//	@Summary	Restore an archived project
//	@Tags		projects
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		key	path		string	true	"Project key"
//	@Success	200	{object}	Project
//	@Router		/api/v1/w/{ws}/projects/{key}/unarchive [post]
func (h *HTTP) unarchive(c *gin.Context) { h.setArchived(c, false) }

// delete hard-deletes an archived project.
//
//	@Summary		Delete a project
//	@Description	Archived projects only, with project.manage. Its issues, their history, configuration overrides and grants go; a ProjectDeleted tombstone remains.
//	@Tags			projects
//	@Param			ws		path	string	true	"Workspace slug"
//	@Param			key		path	string	true	"Project key"
//	@Param			confirm	query	string	true	"The project's current key, typed by the user"
//	@Success		204
//	@Failure		409	{object}	server.Problem	"project_not_archived"
//	@Failure		422	{object}	server.Problem	"confirm: mismatch"
//	@Router			/api/v1/w/{ws}/projects/{key} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if err := h.svc.Delete(c, Current(c), c.Query("confirm")); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *HTTP) setArchived(c *gin.Context, archived bool) {
	p, err := h.svc.SetArchived(c, Current(c), archived)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}
