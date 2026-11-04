package webhook

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves /w/:ws/webhooks.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts the routes on the workspace group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	ws.GET("/webhooks", h.list)
	ws.POST("/webhooks", h.create)
	ws.GET("/webhooks/:id", h.get)
	ws.PATCH("/webhooks/:id", h.update)
	ws.DELETE("/webhooks/:id", h.delete)
	ws.POST("/webhooks/:id/ping", h.ping)
	ws.GET("/webhooks/:id/deliveries", h.deliveries)
	ws.POST("/webhooks/:id/deliveries/:delivery/redeliver", h.redeliver)
}

// WebhookList is the body of GET /webhooks.
type WebhookList struct {
	Items []Webhook `json:"items"`
}

func idOf(c *gin.Context, param, code string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		server.WriteError(c, apperr.NotFound(code))
	}
	return id, err == nil
}

func reply(c *gin.Context, status int, w Webhook, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, w.Version)
	c.JSON(status, w)
}

// list returns the workspace's endpoints.
//
//	@Summary	List webhooks
//	@Tags		webhooks
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	WebhookList
//	@Router		/api/v1/w/{ws}/webhooks [get]
func (h *HTTP) list(c *gin.Context) {
	items, err := h.svc.List(c, workspace.Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, WebhookList{Items: items})
}

// create adds an endpoint.
//
//	@Summary		Create a webhook
//	@Description	workspace.webhooks.manage. The answer carries the signing secret, shown this once (rotate it with PATCH rotate_secret).
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			body	body		Input	true	"Name and URL required"
//	@Success		201		{object}	Webhook
//	@Router			/api/v1/w/{ws}/webhooks [post]
func (h *HTTP) create(c *gin.Context) {
	var in Input
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	w, err := h.svc.Create(c, workspace.Current(c), in)
	reply(c, http.StatusCreated, w, err)
}

// get returns an endpoint.
//
//	@Summary	Get a webhook
//	@Tags		webhooks
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Webhook id"
//	@Success	200	{object}	Webhook
//	@Router		/api/v1/w/{ws}/webhooks/{id} [get]
func (h *HTTP) get(c *gin.Context) {
	if id, ok := idOf(c, "id", "webhook_not_found"); ok {
		w, err := h.svc.Get(c, workspace.Current(c), id)
		reply(c, http.StatusOK, w, err)
	}
}

// update changes an endpoint.
//
//	@Summary	Update a webhook
//	@Tags		webhooks
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		id			path		string	true	"Webhook id"
//	@Param		If-Match	header		string	false	"Webhook version"
//	@Param		body		body		Input	true	"Changes"
//	@Success	200			{object}	Webhook
//	@Router		/api/v1/w/{ws}/webhooks/{id} [patch]
func (h *HTTP) update(c *gin.Context) {
	id, ok := idOf(c, "id", "webhook_not_found")
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
	w, err := h.svc.Update(c, workspace.Current(c), id, version, in)
	reply(c, http.StatusOK, w, err)
}

// delete removes an endpoint and its log.
//
//	@Summary	Delete a webhook
//	@Tags		webhooks
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Webhook id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/webhooks/{id} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if id, ok := idOf(c, "id", "webhook_not_found"); ok {
		if err := h.svc.Delete(c, workspace.Current(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ping queues a test event.
//
//	@Summary	Send a test event
//	@Tags		webhooks
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Webhook id"
//	@Success	202	{object}	Delivery
//	@Router		/api/v1/w/{ws}/webhooks/{id}/ping [post]
func (h *HTTP) ping(c *gin.Context) {
	if id, ok := idOf(c, "id", "webhook_not_found"); ok {
		d, err := h.svc.Ping(c, workspace.Current(c), id)
		if err != nil {
			server.WriteError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, d)
	}
}

// deliveries lists the log.
//
//	@Summary	List a webhook's deliveries
//	@Tags		webhooks
//	@Produce	json
//	@Param		ws		path		string	true	"Workspace slug"
//	@Param		id		path		string	true	"Webhook id"
//	@Param		cursor	query		string	false	"next_cursor of the previous page"
//	@Param		limit	query		int		false	"At most 100; 50 by default"
//	@Success	200		{object}	DeliveryPage
//	@Router		/api/v1/w/{ws}/webhooks/{id}/deliveries [get]
func (h *HTTP) deliveries(c *gin.Context) {
	if id, ok := idOf(c, "id", "webhook_not_found"); ok {
		limit, _ := strconv.Atoi(c.Query("limit"))
		page, err := h.svc.Deliveries(c, workspace.Current(c), id, c.Query("cursor"), limit)
		if err != nil {
			server.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, page)
	}
}

// redeliver queues a delivery again.
//
//	@Summary	Redeliver
//	@Tags		webhooks
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		id			path		string	true	"Webhook id"
//	@Param		delivery	path		string	true	"Delivery id"
//	@Success	202			{object}	Delivery
//	@Router		/api/v1/w/{ws}/webhooks/{id}/deliveries/{delivery}/redeliver [post]
func (h *HTTP) redeliver(c *gin.Context) {
	id, ok := idOf(c, "id", "webhook_not_found")
	if !ok {
		return
	}
	did, ok := idOf(c, "delivery", "delivery_not_found")
	if !ok {
		return
	}
	d, err := h.svc.Redeliver(c, workspace.Current(c), id, did)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, d)
}
