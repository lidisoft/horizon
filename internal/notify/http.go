package notify

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves the inbox, subscriptions and notification settings.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts /me/notifications on the /api/v1 group.
func (h *HTTP) Register(api *gin.RouterGroup) {
	me := api.Group("/me", identity.RequireUser())
	me.GET("/notifications", h.settings)
	me.PUT("/notifications", h.saveSettings)
}

// RegisterWorkspace mounts the inbox and subscriptions on the workspace group.
func (h *HTTP) RegisterWorkspace(ws *gin.RouterGroup) {
	ws.GET("/inbox", h.inbox)
	ws.GET("/inbox/count", h.count)
	ws.POST("/inbox/read-all", h.readAll)
	ws.POST("/inbox/archive-read", h.archiveRead)
	ws.PATCH("/inbox/:id", h.updateItem)
	ws.GET("/issues/:issue/subscription", h.subscription)
	ws.PUT("/issues/:issue/subscription", h.subscribe)
	ws.DELETE("/issues/:issue/subscription", h.unsubscribe)
}

// UnreadCount is the body of GET /inbox/count.
type UnreadCount struct {
	Unread int `json:"unread"`
}

// inbox lists the caller's inbox.
//
//	@Summary		List the inbox
//	@Description	One row per issue the caller follows or was assigned or mentioned on, newest first. state: unread (default), all (read ones too) or snoozed. Rows on issues the caller no longer sees are left out.
//	@Tags			inbox
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			state	query		string	false	"unread, all or snoozed"
//	@Param			cursor	query		string	false	"next_cursor of the previous page"
//	@Param			limit	query		int		false	"At most 100; 50 by default"
//	@Success		200		{object}	Page
//	@Router			/api/v1/w/{ws}/inbox [get]
func (h *HTTP) inbox(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	page, err := h.svc.Inbox(c, workspace.Current(c), c.Query("state"), c.Query("cursor"), limit)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// count counts the caller's unread rows.
//
//	@Summary	Count unread inbox rows
//	@Tags		inbox
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	UnreadCount
//	@Router		/api/v1/w/{ws}/inbox/count [get]
func (h *HTTP) count(c *gin.Context) {
	n, err := h.svc.UnreadCount(c, workspace.Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, UnreadCount{Unread: n})
}

// updateItem changes an inbox row.
//
//	@Summary		Update an inbox row
//	@Description	Mark it read or unread, archive or restore it, or snooze it until a time (null wakes it). New activity on the issue brings it back unread.
//	@Tags			inbox
//	@Accept			json
//	@Param			ws		path	string		true	"Workspace slug"
//	@Param			id		path	string		true	"Row id"
//	@Param			body	body	ItemUpdate	true	"Changes"
//	@Success		204
//	@Router			/api/v1/w/{ws}/inbox/{id} [patch]
func (h *HTTP) updateItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("inbox_item_not_found"))
		return
	}
	var in ItemUpdate
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.UpdateItem(c, workspace.Current(c), id, in); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// readAll marks the inbox read.
//
//	@Summary	Mark the inbox read
//	@Tags		inbox
//	@Param		ws	path	string	true	"Workspace slug"
//	@Success	204
//	@Router		/api/v1/w/{ws}/inbox/read-all [post]
func (h *HTTP) readAll(c *gin.Context) {
	if err := h.svc.ReadAll(c, workspace.Current(c)); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// archiveRead archives the read rows.
//
//	@Summary	Archive the read inbox rows
//	@Tags		inbox
//	@Param		ws	path	string	true	"Workspace slug"
//	@Success	204
//	@Router		/api/v1/w/{ws}/inbox/archive-read [post]
func (h *HTTP) archiveRead(c *gin.Context) {
	if err := h.svc.ArchiveRead(c, workspace.Current(c)); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// subscription tells whether the caller follows the issue.
//
//	@Summary	Get the caller's subscription to an issue
//	@Tags		issues
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	Subscription
//	@Router		/api/v1/w/{ws}/issues/{issue}/subscription [get]
func (h *HTTP) subscription(c *gin.Context) {
	ic, err := h.svc.o.Issues.Enter(c, workspace.Current(c), c.Param("issue"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	sub, err := h.svc.Subscription(c, ic)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, sub)
}

// subscribe makes the caller follow the issue.
//
//	@Summary	Subscribe to an issue
//	@Tags		issues
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		issue	path		string	true	"Issue key"
//	@Success	200		{object}	Subscription
//	@Router		/api/v1/w/{ws}/issues/{issue}/subscription [put]
func (h *HTTP) subscribe(c *gin.Context) {
	ic, err := h.svc.o.Issues.Enter(c, workspace.Current(c), c.Param("issue"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	sub, err := h.svc.Subscribe(c, ic)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, sub)
}

// unsubscribe stops the caller following the issue.
//
//	@Summary		Unsubscribe from an issue
//	@Description	Being assigned or mentioned again subscribes the caller again.
//	@Tags			issues
//	@Param			ws		path	string	true	"Workspace slug"
//	@Param			issue	path	string	true	"Issue key"
//	@Success		204
//	@Router			/api/v1/w/{ws}/issues/{issue}/subscription [delete]
func (h *HTTP) unsubscribe(c *gin.Context) {
	ic, err := h.svc.o.Issues.Enter(c, workspace.Current(c), c.Param("issue"))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.Unsubscribe(c, ic); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// settings returns the caller's email settings.
//
//	@Summary	Get notification settings
//	@Tags		me
//	@Produce	json
//	@Success	200	{object}	Settings
//	@Router		/api/v1/me/notifications [get]
func (h *HTTP) settings(c *gin.Context) {
	p, _ := identity.CurrentPrincipal(c)
	st, err := h.svc.Settings(c, p.User.ID)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// saveSettings stores the caller's email settings.
//
//	@Summary		Set notification settings
//	@Description	email: unread (what stays unread for 10 minutes is emailed) or off; digest: a daily summary in the morning.
//	@Tags			me
//	@Accept			json
//	@Produce		json
//	@Param			body	body		Settings	true	"Settings"
//	@Success		200		{object}	Settings
//	@Router			/api/v1/me/notifications [put]
func (h *HTTP) saveSettings(c *gin.Context) {
	p, _ := identity.CurrentPrincipal(c)
	var in Settings
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	st, err := h.svc.SaveSettings(c, p.User.ID, in)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}
