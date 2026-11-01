package view

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves /w/:ws/views.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts the routes on the workspace group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	ws.GET("/views", h.list)
	ws.POST("/views", h.create)
	ws.GET("/views/:id", h.get)
	ws.PATCH("/views/:id", h.update)
	ws.DELETE("/views/:id", h.delete)
}

// ViewList is the body of GET /views.
type ViewList struct {
	Items []View `json:"items"`
}

// Request creates or changes a view (fields left out are unchanged).
type Request struct {
	Project string          `json:"project,omitempty" example:"HZN"`
	Name    *string         `json:"name,omitempty" example:"Open bugs"`
	Layout  *string         `json:"layout,omitempty" enums:"list,board,table,timeline,calendar"`
	Query   *query.Query    `json:"query,omitempty"`
	Display json.RawMessage `json:"display,omitempty" swaggertype:"object"`
	Shared  *bool           `json:"shared,omitempty"`
}

func reply(c *gin.Context, status int, v View, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, v.Version)
	c.JSON(status, v)
}

func idOf(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("view_not_found"))
	}
	return id, err == nil
}

// list returns the views the caller sees.
//
//	@Summary		List views
//	@Description	Shared views of the workspace and of visible projects, and the caller's own; ?project=KEY narrows to the workspace's and that project's.
//	@Tags			views
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			project	query		string	false	"Project key"
//	@Success		200		{object}	ViewList
//	@Router			/api/v1/w/{ws}/views [get]
func (h *HTTP) list(c *gin.Context) {
	items, err := h.svc.List(c, workspace.Current(c), c.Query("project"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ViewList{Items: items})
}

// create saves a view.
//
//	@Summary		Create a view
//	@Description	Personal by default; shared needs view.share on its project (or the workspace). The query must compile (422 query.<path>).
//	@Tags			views
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			body	body		Request	true	"Name and layout required"
//	@Success		201		{object}	View
//	@Router			/api/v1/w/{ws}/views [post]
func (h *HTTP) create(c *gin.Context) {
	var req Request
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	v, err := h.svc.Create(c, workspace.Current(c), Input(req))
	reply(c, http.StatusCreated, v, err)
}

// get returns one view.
//
//	@Summary	Get a view
//	@Tags		views
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"View id"
//	@Success	200	{object}	View
//	@Router		/api/v1/w/{ws}/views/{id} [get]
func (h *HTTP) get(c *gin.Context) {
	if id, ok := idOf(c); ok {
		v, err := h.svc.Get(c, workspace.Current(c), id)
		reply(c, http.StatusOK, v, err)
	}
}

// update changes a view.
//
//	@Summary		Update a view
//	@Description	Its owner, or view.share for a shared one; sharing or unsharing needs view.share.
//	@Tags			views
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			id			path		string	true	"View id"
//	@Param			If-Match	header		string	false	"View version"
//	@Param			body		body		Request	true	"Changes"
//	@Success		200			{object}	View
//	@Router			/api/v1/w/{ws}/views/{id} [patch]
func (h *HTTP) update(c *gin.Context) {
	id, ok := idOf(c)
	if !ok {
		return
	}
	var req Request
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	v, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	out, err := h.svc.Update(c, workspace.Current(c), id, v, Input(req))
	reply(c, http.StatusOK, out, err)
}

// delete removes a view.
//
//	@Summary	Delete a view
//	@Tags		views
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"View id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/views/{id} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if id, ok := idOf(c); ok {
		if err := h.svc.Delete(c, workspace.Current(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
