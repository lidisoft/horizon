package workspace

import (
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
)

const contextKey = "horizon.workspace"

// HTTP serves the workspace endpoints.
type HTTP struct {
	svc  *Service
	auth *identity.HTTP
}

// NewHTTP returns the handlers; auth signs in people who create their
// account by accepting an invitation.
func NewHTTP(svc *Service, auth *identity.HTTP) *HTTP {
	return &HTTP{svc: svc, auth: auth}
}

// Register mounts the routes on the /api/v1 group, which must run
// identity's Authenticate.
func (h *HTTP) Register(api *gin.RouterGroup) {
	inv := api.Group("/invitations")
	inv.POST("/lookup", h.lookupInvitation)
	inv.POST("/accept", h.acceptInvitation)

	w := api.Group("/w", identity.RequireUser())
	w.GET("", h.list)
	w.POST("", h.create)

	ws := w.Group("/:ws", h.enter)
	ws.GET("", h.get)
	ws.PATCH("", h.update)
	ws.GET("/members", h.members)
	ws.PATCH("/members/:user", h.updateMember)
	ws.DELETE("/members/:user", h.removeMember)
	ws.GET("/invitations", h.invitations)
	ws.POST("/invitations", h.invite)
	ws.POST("/invitations/:id/resend", h.resend)
	ws.DELETE("/invitations/:id", h.revoke)
	ws.GET("/teams", h.teams)
	ws.POST("/teams", h.createTeam)
	ws.PATCH("/teams/:id", h.updateTeam)
	ws.DELETE("/teams/:id", h.deleteTeam)
	ws.PUT("/teams/:id/members/:user", h.addTeamMember)
	ws.DELETE("/teams/:id/members/:user", h.removeTeamMember)
}

// enter resolves /w/:ws for the caller (404 when they may not see it).
func (h *HTTP) enter(c *gin.Context) {
	p, _ := identity.CurrentPrincipal(c)
	wc, err := h.svc.Enter(c, p, c.Param("ws"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.Set(contextKey, wc)
	c.Next()
}

// Current returns the workspace resolved for the request, for other
// modules' handlers under /w/:ws.
func Current(c *gin.Context) Context {
	return c.MustGet(contextKey).(Context)
}

func uuidParam(c *gin.Context, name, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		server.WriteError(c, apperr.NotFound(notFound))
		return uuid.Nil, false
	}
	return id, true
}

// --- Workspaces -----------------------------------------------------------------

// WorkspaceList is the body of GET /w.
type WorkspaceList struct {
	Items []Workspace `json:"items"`
}

// list returns the caller's workspaces.
//
//	@Summary		List workspaces
//	@Description	Workspaces where the caller is an active member; with ?all=true, every workspace (instance admins).
//	@Tags			workspaces
//	@Produce		json
//	@Param			all	query		bool	false	"every workspace of the instance (instance admins)"
//	@Success		200	{object}	WorkspaceList
//	@Router			/api/v1/w [get]
func (h *HTTP) list(c *gin.Context) {
	p, _ := identity.CurrentPrincipal(c)
	var items []Workspace
	var err error
	if c.Query("all") == "true" {
		if !p.User.IsInstanceAdmin || !p.Scope.Includes(identity.ScopeAdmin) {
			server.WriteError(c, apperr.Forbidden("forbidden"))
			return
		}
		items, err = h.svc.All(c)
	} else {
		items, err = h.svc.ForUser(c, p.User.ID)
	}
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, WorkspaceList{Items: items})
}

// CreateWorkspaceRequest is the body of POST /w.
type CreateWorkspaceRequest struct {
	Slug     string `json:"slug" example:"acme"`
	Name     string `json:"name" example:"Acme"`
	TimeZone string `json:"time_zone,omitempty" example:"Europe/Paris"`
}

// create makes a workspace with the caller as first member.
//
//	@Summary		Create a workspace
//	@Description	Instance admins only. The caller becomes the first member.
//	@Tags			workspaces
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateWorkspaceRequest	true	"Slug, name, time zone"
//	@Success		201		{object}	Workspace
//	@Failure		403		{object}	server.Problem
//	@Failure		422		{object}	server.Problem	"slug invalid or taken"
//	@Router			/api/v1/w [post]
func (h *HTTP) create(c *gin.Context) {
	p, _ := identity.CurrentPrincipal(c)
	if !p.User.IsInstanceAdmin || !p.Scope.Includes(identity.ScopeAdmin) {
		server.WriteError(c, apperr.Forbidden("forbidden"))
		return
	}
	var req CreateWorkspaceRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	w, err := h.svc.Create(c, actorOf(p), NewWorkspace{Slug: req.Slug, Name: req.Name, TimeZone: req.TimeZone, OwnerID: p.User.ID})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, w.Version)
	c.JSON(http.StatusCreated, w)
}

// WorkspaceResponse is a workspace with what the caller may do in it.
type WorkspaceResponse struct {
	Workspace
	// Member is false for an instance admin who isn't a member.
	Member bool `json:"member"`
	// Can lists the workspace permissions the caller holds, for the UI.
	Can []string `json:"can"`
}

// get returns one workspace.
//
//	@Summary	Get a workspace
//	@Tags		workspaces
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	WorkspaceResponse
//	@Failure	404	{object}	server.Problem	"workspace_not_found"
//	@Router		/api/v1/w/{ws} [get]
func (h *HTTP) get(c *gin.Context) {
	wc := Current(c)
	can, err := h.svc.Capabilities(c, wc)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, wc.Workspace.Version)
	c.JSON(http.StatusOK, WorkspaceResponse{Workspace: wc.Workspace, Member: wc.Member, Can: can})
}

// UpdateWorkspaceRequest changes the fields that are present.
type UpdateWorkspaceRequest struct {
	Name     *string `json:"name,omitempty"`
	Slug     *string `json:"slug,omitempty"`
	TimeZone *string `json:"time_zone,omitempty"`
}

// update edits a workspace.
//
//	@Summary		Update a workspace
//	@Description	Needs workspace.manage. Changing the slug changes every URL of the workspace.
//	@Tags			workspaces
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string					true	"Workspace slug"
//	@Param			If-Match	header		string					false	"ETag from GET"
//	@Param			body		body		UpdateWorkspaceRequest	true	"Fields to change"
//	@Success		200			{object}	Workspace
//	@Failure		409			{object}	server.Problem	"version_conflict"
//	@Failure		422			{object}	server.Problem
//	@Router			/api/v1/w/{ws} [patch]
func (h *HTTP) update(c *gin.Context) {
	var req UpdateWorkspaceRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	w, err := h.svc.UpdateWorkspace(c, Current(c), version, Update(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, w.Version)
	c.JSON(http.StatusOK, w)
}

// --- Members --------------------------------------------------------------------

// MemberList is the body of GET /w/{ws}/members.
type MemberList struct {
	Items []Member `json:"items"`
}

// members lists the workspace's members.
//
//	@Summary	List members
//	@Tags		members
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	MemberList
//	@Router		/api/v1/w/{ws}/members [get]
func (h *HTTP) members(c *gin.Context) {
	items, err := h.svc.Members(c, Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemberList{Items: items})
}

// UpdateMemberRequest is the body of PATCH /w/{ws}/members/{user}.
type UpdateMemberRequest struct {
	Status string `json:"status" enums:"active,suspended"`
}

// updateMember suspends or reactivates a member.
//
//	@Summary		Suspend or reactivate a member
//	@Description	Needs workspace.members.manage. Suspension removes access to this workspace only.
//	@Tags			members
//	@Accept			json
//	@Param			ws		path	string				true	"Workspace slug"
//	@Param			user	path	string				true	"User id"
//	@Param			body	body	UpdateMemberRequest	true	"New status"
//	@Success		204
//	@Failure		409	{object}	server.Problem	"cannot_change_self"
//	@Router			/api/v1/w/{ws}/members/{user} [patch]
func (h *HTTP) updateMember(c *gin.Context) {
	id, ok := uuidParam(c, "user", "member_not_found")
	if !ok {
		return
	}
	var req UpdateMemberRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	if req.Status != statusActive && req.Status != statusSuspended {
		server.WriteError(c, apperr.Invalid(apperr.Field{Field: "status", Code: "invalid"}))
		return
	}
	if err := h.svc.SetSuspended(c, Current(c), id, req.Status == statusSuspended); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// removeMember takes a member out of the workspace.
//
//	@Summary	Remove a member
//	@Tags		members
//	@Param		ws		path	string	true	"Workspace slug"
//	@Param		user	path	string	true	"User id"
//	@Success	204
//	@Failure	409	{object}	server.Problem	"cannot_change_self"
//	@Router		/api/v1/w/{ws}/members/{user} [delete]
func (h *HTTP) removeMember(c *gin.Context) {
	id, ok := uuidParam(c, "user", "member_not_found")
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(c, Current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Invitations ----------------------------------------------------------------

// InvitationList is the body of GET and POST /w/{ws}/invitations.
type InvitationList struct {
	Items []Invitation `json:"items"`
}

// invitations lists open invitations.
//
//	@Summary	List pending invitations
//	@Tags		members
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	InvitationList
//	@Router		/api/v1/w/{ws}/invitations [get]
func (h *HTTP) invitations(c *gin.Context) {
	items, err := h.svc.Invitations(c, Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, InvitationList{Items: items})
}

// InviteRequest is the body of POST /w/{ws}/invitations.
type InviteRequest struct {
	Emails  []string    `json:"emails"`
	TeamIDs []uuid.UUID `json:"team_ids,omitempty"`
}

// invite sends invitations.
//
//	@Summary		Invite people
//	@Description	Up to 50 emails. An email with a pending invitation gets it renewed and sent again.
//	@Tags			members
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		InviteRequest	true	"Emails and teams to join"
//	@Success		201		{object}	InvitationList
//	@Failure		422		{object}	server.Problem	"emails[i] invalid or already_member, team_ids unknown"
//	@Router			/api/v1/w/{ws}/invitations [post]
func (h *HTTP) invite(c *gin.Context) {
	var req InviteRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	items, err := h.svc.InvitePeople(c, Current(c), Invite{Emails: req.Emails, TeamIDs: req.TeamIDs})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, InvitationList{Items: items})
}

// resend renews and resends an invitation.
//
//	@Summary	Resend an invitation
//	@Tags		members
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Invitation id"
//	@Success	200	{object}	Invitation
//	@Failure	404	{object}	server.Problem	"invitation_not_found"
//	@Router		/api/v1/w/{ws}/invitations/{id}/resend [post]
func (h *HTTP) resend(c *gin.Context) {
	id, ok := uuidParam(c, "id", "invitation_not_found")
	if !ok {
		return
	}
	inv, err := h.svc.Resend(c, Current(c), id)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

// revoke cancels an invitation.
//
//	@Summary	Revoke an invitation
//	@Tags		members
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Invitation id"
//	@Success	204
//	@Failure	404	{object}	server.Problem	"invitation_not_found"
//	@Router		/api/v1/w/{ws}/invitations/{id} [delete]
func (h *HTTP) revoke(c *gin.Context) {
	id, ok := uuidParam(c, "id", "invitation_not_found")
	if !ok {
		return
	}
	if err := h.svc.Revoke(c, Current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// InvitationTokenRequest carries an invitation token in the body, so it
// stays out of URLs and logs.
type InvitationTokenRequest struct {
	Token string `json:"token"`
}

// lookupInvitation describes an invitation before accepting it.
//
//	@Summary	Look up an invitation
//	@Tags		invitations
//	@Accept		json
//	@Produce	json
//	@Param		body	body		InvitationTokenRequest	true	"Token from the emailed link"
//	@Success	200		{object}	InvitationPreview
//	@Failure	404		{object}	server.Problem	"invitation_invalid: unknown, used, revoked or expired"
//	@Router		/api/v1/invitations/lookup [post]
func (h *HTTP) lookupInvitation(c *gin.Context) {
	var req InvitationTokenRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	preview, err := h.svc.Preview(c, req.Token)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

// AcceptInvitationRequest is the body of POST /invitations/accept. Name and
// password create the account when the email has none.
type AcceptInvitationRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

// AcceptedResponse names the workspace joined.
type AcceptedResponse struct {
	Workspace Workspace `json:"workspace"`
}

// acceptInvitation joins the workspace.
//
//	@Summary		Accept an invitation
//	@Description	Signed in, the account's email must be the invited one. Signed out, name and password create the account and sign it in; an existing account must sign in first.
//	@Tags			invitations
//	@Accept			json
//	@Produce		json
//	@Param			body	body		AcceptInvitationRequest	true	"Token, and name and password for a new account"
//	@Success		200		{object}	AcceptedResponse
//	@Failure		401		{object}	server.Problem	"sign_in_required"
//	@Failure		403		{object}	server.Problem	"email_mismatch"
//	@Failure		404		{object}	server.Problem	"invitation_invalid"
//	@Router			/api/v1/invitations/accept [post]
func (h *HTTP) acceptInvitation(c *gin.Context) {
	var req AcceptInvitationRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	in := Acceptance{Token: req.Token, Name: req.Name, Password: req.Password}
	if p, ok := identity.CurrentPrincipal(c); ok {
		if p.SessionID == uuid.Nil {
			server.WriteError(c, apperr.Forbidden("session_required"))
			return
		}
		in.Principal = &p
	}
	ip, _ := netip.ParseAddr(c.ClientIP())
	in.Client = identity.Client{UserAgent: c.Request.UserAgent(), IP: ip}
	out, err := h.svc.Accept(c, in)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	if out.SignedIn != nil {
		h.auth.SetSession(c, *out.SignedIn)
	}
	c.JSON(http.StatusOK, AcceptedResponse{Workspace: out.Workspace})
}

// --- Teams ----------------------------------------------------------------------

// TeamList is the body of GET /w/{ws}/teams.
type TeamList struct {
	Items []Team `json:"items"`
}

// teams lists teams.
//
//	@Summary	List teams
//	@Tags		teams
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	TeamList
//	@Router		/api/v1/w/{ws}/teams [get]
func (h *HTTP) teams(c *gin.Context) {
	items, err := h.svc.Teams(c, Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TeamList{Items: items})
}

// TeamRequest is the body of POST and PATCH /w/{ws}/teams.
type TeamRequest struct {
	Name        *string `json:"name,omitempty" example:"Core"`
	Description *string `json:"description,omitempty"`
}

// createTeam adds a team.
//
//	@Summary	Create a team
//	@Tags		teams
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string		true	"Workspace slug"
//	@Param		body	body		TeamRequest	true	"Name and description"
//	@Success	201		{object}	Team
//	@Failure	422		{object}	server.Problem	"name taken"
//	@Router		/api/v1/w/{ws}/teams [post]
func (h *HTTP) createTeam(c *gin.Context) {
	var req TeamRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	t, err := h.svc.CreateTeam(c, Current(c), TeamInput(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

// updateTeam renames a team or changes its description.
//
//	@Summary	Update a team
//	@Tags		teams
//	@Accept		json
//	@Param		ws			path	string		true	"Workspace slug"
//	@Param		id			path	string		true	"Team id"
//	@Param		If-Match	header	string		false	"Team version"
//	@Param		body		body	TeamRequest	true	"Fields to change"
//	@Success	204
//	@Router		/api/v1/w/{ws}/teams/{id} [patch]
func (h *HTTP) updateTeam(c *gin.Context) {
	id, ok := uuidParam(c, "id", "team_not_found")
	if !ok {
		return
	}
	var req TeamRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.UpdateTeam(c, Current(c), id, version, TeamInput(req)); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deleteTeam removes a team.
//
//	@Summary	Delete a team
//	@Tags		teams
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Team id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/teams/{id} [delete]
func (h *HTTP) deleteTeam(c *gin.Context) {
	id, ok := uuidParam(c, "id", "team_not_found")
	if !ok {
		return
	}
	if err := h.svc.DeleteTeam(c, Current(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// addTeamMember puts a member in a team.
//
//	@Summary	Add a member to a team
//	@Tags		teams
//	@Param		ws		path	string	true	"Workspace slug"
//	@Param		id		path	string	true	"Team id"
//	@Param		user	path	string	true	"User id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/teams/{id}/members/{user} [put]
func (h *HTTP) addTeamMember(c *gin.Context) { h.setTeamMember(c, true) }

// removeTeamMember takes a member out of a team.
//
//	@Summary	Remove a member from a team
//	@Tags		teams
//	@Param		ws		path	string	true	"Workspace slug"
//	@Param		id		path	string	true	"Team id"
//	@Param		user	path	string	true	"User id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/teams/{id}/members/{user} [delete]
func (h *HTTP) removeTeamMember(c *gin.Context) { h.setTeamMember(c, false) }

func (h *HTTP) setTeamMember(c *gin.Context, member bool) {
	team, ok := uuidParam(c, "id", "team_not_found")
	if !ok {
		return
	}
	user, ok := uuidParam(c, "user", "member_not_found")
	if !ok {
		return
	}
	if err := h.svc.SetTeamMember(c, Current(c), team, user, member); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
