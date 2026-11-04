package sso

import (
	"net/http"
	"net/netip"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves the SSO sign-in and the providers' management.
type HTTP struct {
	svc    *Service
	auth   *identity.HTTP
	secure bool
}

// NewHTTP returns the handlers; auth sets the session cookie, secure is
// true over HTTPS.
func NewHTTP(svc *Service, auth *identity.HTTP, secure bool) *HTTP {
	return &HTTP{svc: svc, auth: auth, secure: secure}
}

// Register mounts the sign-in routes and the instance's providers on
// /api/v1.
func (h *HTTP) Register(api *gin.RouterGroup) {
	auth := api.Group("/auth/oidc")
	auth.GET("/providers", h.signInProviders)
	auth.POST("/discover", h.discover)
	auth.GET("/:slug/start", h.start)
	auth.GET("/:slug/callback", h.callback)
	admin := api.Group("/admin/oidc-providers", identity.RequireUser())
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.PATCH("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

// RegisterWorkspace mounts the workspace's providers and enforcement.
func (h *HTTP) RegisterWorkspace(ws *gin.RouterGroup) {
	ws.GET("/sso/providers", h.list)
	ws.POST("/sso/providers", h.create)
	ws.PATCH("/sso/providers/:id", h.update)
	ws.DELETE("/sso/providers/:id", h.delete)
	ws.GET("/sso", h.enforcement)
	ws.PUT("/sso", h.setEnforcement)
}

// scope is the workspace of the request, or the instance.
func scope(c *gin.Context) Scope {
	if c.Param("ws") != "" {
		wc := workspace.Current(c)
		return Scope{Workspace: &wc}
	}
	p, _ := identity.CurrentPrincipal(c)
	return Scope{Principal: p}
}

func (h *HTTP) stateCookie() string {
	if h.secure {
		return "__Host-horizon_oidc"
	}
	return "horizon_oidc"
}

// ProviderList is the body of GET …/sso/providers.
type ProviderList struct {
	Items []Provider `json:"items"`
}

// SignInProviderList is the body of GET /auth/oidc/providers.
type SignInProviderList struct {
	Items []SignInProvider `json:"items"`
}

// signInProviders lists the providers of the sign-in page.
//
//	@Summary	List the sign-in providers
//	@Tags		auth
//	@Produce	json
//	@Success	200	{object}	SignInProviderList
//	@Router		/api/v1/auth/oidc/providers [get]
func (h *HTTP) signInProviders(c *gin.Context) {
	items, err := h.svc.SignInProviders(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, SignInProviderList{Items: items})
}

// DiscoverRequest is the body of POST /auth/oidc/discover.
type DiscoverRequest struct {
	Email string `json:"email" example:"ada@acme.example"`
}

// discover finds the provider for an email's domain.
//
//	@Summary	Find the SSO provider for an email
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		DiscoverRequest	true	"Email"
//	@Success	200		{object}	SignInProvider
//	@Failure	404		{object}	server.Problem	"no_sso"
//	@Router		/api/v1/auth/oidc/discover [post]
func (h *HTTP) discover(c *gin.Context) {
	var req DiscoverRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	p, err := h.svc.Discover(c, req.Email)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// start sends the browser to the provider.
//
//	@Summary	Start an SSO sign-in
//	@Tags		auth
//	@Param		slug	path	string	true	"Provider"
//	@Param		next	query	string	false	"Where to land afterwards (a path)"
//	@Success	302
//	@Router		/api/v1/auth/oidc/{slug}/start [get]
func (h *HTTP) start(c *gin.Context) {
	target, cookie, err := h.svc.Start(c, c.Param("slug"), c.Query("next"))
	if err != nil {
		h.fail(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: h.stateCookie(), Value: cookie, Path: "/", MaxAge: int(stateTTL.Seconds()),
		Secure: h.secure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	c.Redirect(http.StatusFound, target)
}

// callback finishes the sign-in and lands where it started.
//
//	@Summary		Finish an SSO sign-in
//	@Description	The provider's redirect. Success sets the session cookie and redirects to the page the sign-in started from; failure redirects to /sign-in?sso_error=<code>.
//	@Tags			auth
//	@Param			slug	path	string	true	"Provider"
//	@Success		302
//	@Router			/api/v1/auth/oidc/{slug}/callback [get]
func (h *HTTP) callback(c *gin.Context) {
	cookie, _ := c.Cookie(h.stateCookie())
	http.SetCookie(c.Writer, &http.Cookie{Name: h.stateCookie(), Value: "", Path: "/", MaxAge: -1, Secure: h.secure, HttpOnly: true,
		SameSite: http.SameSiteLaxMode})
	if c.Query("error") != "" {
		h.fail(c, apperr.Unauthorized("sso_cancelled"))
		return
	}
	ip, _ := netip.ParseAddr(c.ClientIP())
	in, next, err := h.svc.Finish(c, c.Param("slug"), cookie, c.Query("state"), c.Query("code"),
		identity.Client{UserAgent: c.Request.UserAgent(), IP: ip})
	if err != nil {
		h.fail(c, err)
		return
	}
	h.auth.SetSession(c, in)
	c.Redirect(http.StatusFound, next)
}

// fail sends the browser back to the sign-in page with the error's code.
func (h *HTTP) fail(c *gin.Context, err error) {
	code := "sso_failed"
	if e, ok := apperr.As(err); ok {
		code = e.Code
		if e.Code == "bad_request" {
			code = "sso_failed"
		}
		if e.Detail == "sso_state" {
			code = "sso_state"
		}
	} else {
		log := server.Log(c)
		log.Error().Err(err).Msg("sso sign-in")
	}
	c.Redirect(http.StatusFound, "/sign-in?sso_error="+url.QueryEscape(code))
}

func idOf(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("provider_not_found"))
	}
	return id, err == nil
}

func reply(c *gin.Context, status int, p Provider, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, p.Version)
	c.JSON(status, p)
}

// list returns the providers.
//
//	@Summary		List OIDC providers
//	@Description	A workspace's (workspace.manage) at /w/{ws}/sso/providers; the instance's (instance admins) at /admin/oidc-providers.
//	@Tags			sso
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	ProviderList
//	@Router			/api/v1/w/{ws}/sso/providers [get]
func (h *HTTP) list(c *gin.Context) {
	items, err := h.svc.List(c, scope(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ProviderList{Items: items})
}

// create adds a provider.
//
//	@Summary		Create an OIDC provider
//	@Description	Register redirect_uri at the provider. provision_role_id (workspace providers) creates accounts and memberships just in time.
//	@Tags			sso
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string	true	"Workspace slug"
//	@Param			body	body		Input	true	"slug, name, issuer, client_id and client_secret required"
//	@Success		201		{object}	Provider
//	@Router			/api/v1/w/{ws}/sso/providers [post]
func (h *HTTP) create(c *gin.Context) {
	var in Input
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	p, err := h.svc.Create(c, scope(c), in)
	reply(c, http.StatusCreated, p, err)
}

// update changes a provider.
//
//	@Summary	Update an OIDC provider
//	@Tags		sso
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string	true	"Workspace slug"
//	@Param		id			path		string	true	"Provider id"
//	@Param		If-Match	header		string	false	"Provider version"
//	@Param		body		body		Input	true	"Changes"
//	@Success	200			{object}	Provider
//	@Router		/api/v1/w/{ws}/sso/providers/{id} [patch]
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
	p, err := h.svc.Update(c, scope(c), id, version, in)
	reply(c, http.StatusOK, p, err)
}

// delete removes a provider.
//
//	@Summary	Delete an OIDC provider
//	@Tags		sso
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Provider id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/sso/providers/{id} [delete]
func (h *HTTP) delete(c *gin.Context) {
	if id, ok := idOf(c); ok {
		if err := h.svc.Delete(c, scope(c), id); err != nil {
			server.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// enforcement tells whether the workspace requires SSO.
//
//	@Summary	Get the SSO requirement
//	@Tags		sso
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	Enforcement
//	@Router		/api/v1/w/{ws}/sso [get]
func (h *HTTP) enforcement(c *gin.Context) {
	e, err := h.svc.Enforcement(c, workspace.Current(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

// setEnforcement turns the SSO requirement on or off.
//
//	@Summary		Require SSO
//	@Description	workspace.manage. On, members sign in with an OIDC provider only (instance admins keep their password), and their password and magic-link sessions close; it needs an enabled provider (409 no_sso_provider).
//	@Tags			sso
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string		true	"Workspace slug"
//	@Param			body	body		Enforcement	true	"enforced"
//	@Success		200		{object}	Enforcement
//	@Router			/api/v1/w/{ws}/sso [put]
func (h *HTTP) setEnforcement(c *gin.Context) {
	var in Enforcement
	if err := server.BindJSON(c, &in); err != nil {
		server.WriteError(c, err)
		return
	}
	e, err := h.svc.SetEnforcement(c, workspace.Current(c), in.Enforced)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}
