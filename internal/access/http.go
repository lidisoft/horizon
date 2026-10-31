package access

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
)

const scopeKey = "horizon.access.scope"

// SetScope records the workspace (and project) a request acts in; the
// module that resolves /w/:ws calls it.
func SetScope(c *gin.Context, s Scope) { c.Set(scopeKey, s) }

// ScopeOf returns the scope recorded by SetScope.
func ScopeOf(c *gin.Context) Scope { return c.MustGet(scopeKey).(Scope) }

// Need is route middleware declaring the permission a handler requires
// (docs/security.md#enforcement-points); it runs before the handler loads
// anything.
func (s *Service) Need(perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, _ := identity.CurrentPrincipal(c)
		if err := Require(c, s, p, ScopeOf(c), perm); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Next()
	}
}

// HTTP serves roles and grants under /w/:ws.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts the routes on the /w/:ws group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	manage := h.svc.Need(WorkspaceRolesManage)
	ws.GET("/roles", h.roles)
	ws.POST("/roles", manage, h.createRole)
	ws.PATCH("/roles/:id", manage, h.updateRole)
	ws.DELETE("/roles/:id", manage, h.deleteRole)
	ws.GET("/grants", h.grants)
	ws.PUT("/grants", manage, h.setGrant)
	ws.DELETE("/grants/:id", manage, h.removeGrant)
}

func actor(c *gin.Context) uuid.NullUUID {
	p, _ := identity.CurrentPrincipal(c)
	return uuid.NullUUID{UUID: p.User.ID, Valid: p.User.ID != uuid.Nil}
}

// seesWorkspace answers 403 to guests for the workspace-wide lists.
func (h *HTTP) seesWorkspace(c *gin.Context) bool {
	p, _ := identity.CurrentPrincipal(c)
	ok, err := h.svc.SeesWorkspace(c, p, ScopeOf(c).WorkspaceID)
	if err == nil && !ok {
		err = apperr.Forbidden("forbidden")
	}
	if err != nil {
		server.WriteError(c, err)
		return false
	}
	return true
}

// RolesResponse lists the roles and the permission catalogue.
type RolesResponse struct {
	Items     []Role `json:"items"`
	Catalogue []Area `json:"catalogue"`
}

// roles lists roles with the catalogue, for the permission matrix.
//
//	@Summary	List roles
//	@Tags		roles
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	RolesResponse
//	@Router		/api/v1/w/{ws}/roles [get]
func (h *HTTP) roles(c *gin.Context) {
	if !h.seesWorkspace(c) {
		return
	}
	items, err := h.svc.Roles(c, ScopeOf(c).WorkspaceID)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, RolesResponse{Items: items, Catalogue: Catalogue})
}

// RoleRequest is the body of POST and PATCH /roles.
type RoleRequest struct {
	Name        *string       `json:"name,omitempty" example:"QA"`
	Description *string       `json:"description,omitempty"`
	Permissions *[]Permission `json:"permissions,omitempty"`
}

// createRole adds a custom role.
//
//	@Summary	Create a role
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string		true	"Workspace slug"
//	@Param		body	body		RoleRequest	true	"Name, description, permissions"
//	@Success	201		{object}	Role
//	@Failure	422		{object}	server.Problem	"name taken, permissions unknown"
//	@Router		/api/v1/w/{ws}/roles [post]
func (h *HTTP) createRole(c *gin.Context) {
	var req RoleRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	r, err := h.svc.CreateRole(c, ScopeOf(c).WorkspaceID, actor(c), RoleInput(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, r)
}

// updateRole renames a role or replaces its permissions.
//
//	@Summary		Update a role
//	@Description	The Admin role keeps workspace.roles.manage (409 admin_keeps_roles_manage).
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string		true	"Workspace slug"
//	@Param			id			path		string		true	"Role id"
//	@Param			If-Match	header		string		false	"Role version"
//	@Param			body		body		RoleRequest	true	"Fields to change"
//	@Success		200			{object}	Role
//	@Failure		409			{object}	server.Problem
//	@Router			/api/v1/w/{ws}/roles/{id} [patch]
func (h *HTTP) updateRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("role_not_found"))
		return
	}
	var req RoleRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	r, err := h.svc.UpdateRole(c, ScopeOf(c).WorkspaceID, actor(c), id, version, RoleInput(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// deleteRole removes a role nobody holds.
//
//	@Summary		Delete a role
//	@Description	Refused for the Admin role and for roles still granted (409 role_in_use).
//	@Tags			roles
//	@Param			ws	path	string	true	"Workspace slug"
//	@Param			id	path	string	true	"Role id"
//	@Success		204
//	@Failure		409	{object}	server.Problem
//	@Router			/api/v1/w/{ws}/roles/{id} [delete]
func (h *HTTP) deleteRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("role_not_found"))
		return
	}
	if err := h.svc.DeleteRole(c, ScopeOf(c).WorkspaceID, actor(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GrantList is the body of GET /grants.
type GrantList struct {
	Items []Grant `json:"items"`
}

// grants lists every grant of the workspace.
//
//	@Summary	List grants
//	@Tags		roles
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	GrantList
//	@Router		/api/v1/w/{ws}/grants [get]
func (h *HTTP) grants(c *gin.Context) {
	if !h.seesWorkspace(c) {
		return
	}
	items, err := h.svc.Grants(c, ScopeOf(c).WorkspaceID)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, GrantList{Items: items})
}

// GrantRequest sets the role of a member or a team in a scope.
type GrantRequest struct {
	RoleID    uuid.UUID  `json:"role_id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	TeamID    *uuid.UUID `json:"team_id,omitempty"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
}

func null(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

// setGrant gives a role, replacing the subject's previous role in the scope.
//
//	@Summary		Set a grant
//	@Description	One of user_id and team_id. Without project_id the grant covers the whole workspace. Removing the last workspace Admin is refused (409 last_admin).
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		GrantRequest	true	"Role, subject and scope"
//	@Success		200		{object}	Grant
//	@Failure		409		{object}	server.Problem	"last_admin"
//	@Failure		422		{object}	server.Problem
//	@Router			/api/v1/w/{ws}/grants [put]
func (h *HTTP) setGrant(c *gin.Context) {
	var req GrantRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	g, err := h.svc.SetGrant(c, ScopeOf(c).WorkspaceID, actor(c), GrantInput{
		RoleID: req.RoleID, UserID: null(req.UserID), TeamID: null(req.TeamID), ProjectID: null(req.ProjectID),
	})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

// removeGrant takes a role away.
//
//	@Summary	Remove a grant
//	@Tags		roles
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Grant id"
//	@Success	204
//	@Failure	409	{object}	server.Problem	"last_admin"
//	@Router		/api/v1/w/{ws}/grants/{id} [delete]
func (h *HTTP) removeGrant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("grant_not_found"))
		return
	}
	if err := h.svc.RemoveGrant(c, ScopeOf(c).WorkspaceID, actor(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
