package vcs

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/vcs/db"
	"github.com/lidisoft/horizon/internal/workspace"
)

// maxDelivery bounds an inbound delivery's body.
const maxDelivery = 5 << 20

// HTTP serves the connections, the inbound hooks and an issue's links.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// RegisterHooks mounts the inbound forge webhooks on /api/v1. Register them
// before the authentication middleware: forges sign the body instead, and
// may send an Authorization header of their own.
func (h *HTTP) RegisterHooks(api *gin.RouterGroup) {
	api.POST("/hooks/vcs/:id", h.receive)
}

// Register mounts the connections and the issues' links on the workspace
// group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	ws.GET("/integrations/vcs", h.list)
	ws.POST("/integrations/vcs", h.create)
	ws.GET("/integrations/vcs/:id", h.get)
	ws.PATCH("/integrations/vcs/:id", h.update)
	ws.DELETE("/integrations/vcs/:id", h.delete)
	ws.GET("/issues/:issue/git", h.links)
}

// ConnectionList is the body of GET /integrations/vcs.
type ConnectionList struct {
	Items []Connection `json:"items"`
}

func idOf(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("connection_not_found"))
	}
	return id, err == nil
}

func reply(c *gin.Context, status int, v Connection, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, v.Version)
	c.JSON(status, v)
}

// receive takes a forge's webhook delivery.
//
//	@Summary		Receive a forge webhook
//	@Description	GitHub, GitLab and Forgejo deliveries for a connection, checked against its secret (X-Hub-Signature-256, X-Forgejo-Signature or X-Gitlab-Token). Push, branch and pull request events link the issues they name; others are accepted and ignored.
//	@Tags			integrations
//	@Accept			json
//	@Param			id	path	string	true	"Connection id"
//	@Success		204
//	@Failure		401	{object}	server.Problem
//	@Router			/api/v1/hooks/vcs/{id} [post]
func (h *HTTP) receive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("connection_not_found"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxDelivery+1))
	if err != nil || len(body) > maxDelivery {
		server.WriteError(c, apperr.BadRequest("delivery too large"))
		return
	}
	// The forge may hang up: finish what it delivered.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c), 30*time.Second)
	defer cancel()
	if err := h.svc.Receive(ctx, id, c.Request.Header, body); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// list returns the connections.
//
//	@Summary	List Git connections
//	@Tags		integrations
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	ConnectionList
//	@Router		/api/v1/w/{ws}/integrations/vcs [get]
func (h *HTTP) list(c *gin.Context) {
	items, err := h.svc.List(c, workspace.Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ConnectionList{Items: items})
}

// create adds a connection.
//
//	@Summary		Create a Git connection
//	@Description	workspace.integrations.manage. The answer carries the webhook URL and secret to set on the forge, the secret shown this once.
//	@Tags			integrations
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			body	body		Input	true	"Provider, name and base_url required"
//	@Success		201		{object}	Connection
//	@Router			/api/v1/w/{ws}/integrations/vcs [post]
func (h *HTTP) create(c *gin.Context) {
	var in Input
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	v, err := h.svc.Create(c, workspace.Current(c), in)
	reply(c, http.StatusCreated, v, err)
}

// get returns a connection.
//
//	@Summary	Get a Git connection
//	@Tags		integrations
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Connection id"
//	@Success	200	{object}	Connection
//	@Router		/api/v1/w/{ws}/integrations/vcs/{id} [get]
func (h *HTTP) get(c *gin.Context) {
	if id, ok := idOf(c); ok {
		v, err := h.svc.Get(c, workspace.Current(c), id)
		reply(c, http.StatusOK, v, err)
	}
}

// update changes a connection.
//
//	@Summary	Update a Git connection
//	@Tags		integrations
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		id			path		string	true	"Connection id"
//	@Param		If-Match	header		string	false	"Connection version"
//	@Param		body		body		Input	true	"Changes"
//	@Success	200			{object}	Connection
//	@Router		/api/v1/w/{ws}/integrations/vcs/{id} [patch]
func (h *HTTP) update(c *gin.Context) {
	id, ok := idOf(c)
	if !ok {
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	var in Input
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	v, err := h.svc.Update(c, workspace.Current(c), id, version, in)
	reply(c, http.StatusOK, v, err)
}

// delete removes a connection and its links.
//
//	@Summary	Delete a Git connection
//	@Tags		integrations
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Connection id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/integrations/vcs/{id} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if id, ok := idOf(c); ok {
		if err := h.svc.Delete(c, workspace.Current(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// Link is a branch, pull request or commit naming an issue.
type Link struct {
	Kind       string `json:"kind" enums:"branch,pull_request,commit"`
	Provider   string `json:"provider" enums:"github,gitlab,forgejo"`
	Repository string `json:"repository" example:"acme/horizon"`
	// Ref is the branch name, the pull request number or the commit hash.
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	State  string `json:"state" enums:"open,merged,closed,deleted,"`
	Author string `json:"author"`
	// UpdatedAt is when the forge last told of it.
	UpdatedAt time.Time `json:"updated_at"`
}

// LinkList is the body of GET /issues/{key}/git.
type LinkList struct {
	Items []Link `json:"items"`
}

// links lists an issue's Git links.
//
//	@Summary		List an issue's branches, pull requests and commits
//	@Description	Pull requests first, then branches, then commits; newest first.
//	@Tags			issues
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			issue	path		string	true	"Issue key"
//	@Success		200		{object}	LinkList
//	@Router			/api/v1/w/{ws}/issues/{issue}/git [get]
func (h *HTTP) links(c *gin.Context) {
	ic, err := h.svc.o.Issues.Enter(c, workspace.Current(c), c.Param("issue"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	items, err := h.svc.Links(c, ic)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, LinkList{Items: items})
}

// Links lists the issue's links (the caller sees the issue).
func (s *Service) Links(ctx context.Context, ic issue.Context) ([]Link, error) {
	rows, err := db.New(s.o.Pool).IssueLinks(ctx, ic.Issue.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Link, len(rows))
	for i, r := range rows {
		out[i] = Link{Kind: r.Kind, Provider: r.Provider, Repository: r.Repository, Ref: r.ExternalID, Title: r.Title, URL: r.Url,
			State: r.State, Author: r.Author, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}
