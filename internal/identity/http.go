package identity

import (
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/server"
)

// HeaderCSRF carries the CSRF token on state-changing requests made with the
// session cookie.
const HeaderCSRF = "X-CSRF-Token"

const (
	principalKey  = "horizon.principal"
	csrfFailedKey = "horizon.csrf_failed"
)

// HTTP serves the identity endpoints and authenticates requests.
type HTTP struct {
	svc    *Service
	secure bool // HTTPS: Secure cookie with the __Host- prefix
}

// NewHTTP returns the handlers. secure is true when BASE_URL is https.
func NewHTTP(svc *Service, secure bool) *HTTP {
	return &HTTP{svc: svc, secure: secure}
}

// cookieName is the session cookie. Over HTTPS the __Host- prefix pins it to
// this exact origin.
func (h *HTTP) cookieName() string {
	if h.secure {
		return "__Host-horizon_session"
	}
	return "horizon_session"
}

func (h *HTTP) setCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *HTTP) clearCookie(c *gin.Context) { h.setCookie(c, "", -1) }

// Authenticate resolves the session cookie into a Principal for the
// handlers. It never rejects by itself: RequireUser does. A state-changing
// request whose CSRF token doesn't match is treated as anonymous and flagged,
// so RequireUser answers csrf_invalid.
//
// An Authorization header takes precedence over the cookie and must be a
// valid "Bearer hzn_…" token, else the request is rejected outright. A token
// without the write scope can only make safe requests.
func (h *HTTP) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if header := c.GetHeader("Authorization"); header != "" {
			h.authenticateBearer(c, header)
			return
		}
		token, err := c.Cookie(h.cookieName())
		if err != nil || token == "" {
			c.Next()
			return
		}
		p, err := h.svc.Authenticate(c, token)
		switch {
		case apperr.Is(err, "unauthenticated"):
			h.clearCookie(c)
		case err != nil:
			server.WriteError(c, err)
			return
		case !safeMethod(c.Request.Method) && !h.svc.ValidCSRF(p, c.GetHeader(HeaderCSRF)):
			c.Set(csrfFailedKey, true)
		default:
			if p.renewed {
				h.setCookie(c, token, int(SessionIdleTimeout.Seconds()))
			}
			c.Set(principalKey, p)
		}
		c.Next()
	}
}

func (h *HTTP) authenticateBearer(c *gin.Context, header string) {
	scheme, token, _ := strings.Cut(header, " ")
	if !strings.EqualFold(scheme, "Bearer") {
		bearerChallenge(c, apperr.Unauthorized("unauthenticated"))
		return
	}
	p, err := h.svc.AuthenticateToken(c, strings.TrimSpace(token))
	if err != nil {
		bearerChallenge(c, err)
		return
	}
	if !safeMethod(c.Request.Method) && !p.Scope.Includes(ScopeWrite) {
		server.WriteError(c, apperr.Forbidden("insufficient_scope"))
		return
	}
	c.Set(principalKey, p)
	c.Next()
}

// bearerChallenge answers a failed bearer authentication (RFC 6750).
func bearerChallenge(c *gin.Context, err error) {
	if apperr.Is(err, "unauthenticated") {
		c.Header("WWW-Authenticate", `Bearer realm="horizon", error="invalid_token"`)
	}
	server.WriteError(c, err)
}

// RequireSession rejects API tokens: managing credentials (password,
// sessions, tokens) needs a signed-in browser.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p, ok := CurrentPrincipal(c); ok && p.SessionID == uuid.Nil {
			server.WriteError(c, apperr.Forbidden("session_required"))
			return
		}
		c.Next()
	}
}

// RequireScope rejects tokens without need; sessions have every scope.
func RequireScope(need Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p, ok := CurrentPrincipal(c); ok && !p.Scope.Includes(need) {
			server.WriteError(c, apperr.Forbidden("insufficient_scope"))
			return
		}
		c.Next()
	}
}

// RequireUser rejects anonymous requests with 401 (403 for a CSRF failure).
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentPrincipal(c); ok {
			c.Next()
			return
		}
		if c.GetBool(csrfFailedKey) {
			server.WriteError(c, apperr.Forbidden("csrf_invalid"))
			return
		}
		server.WriteError(c, apperr.Unauthorized("unauthenticated"))
	}
}

// CurrentPrincipal returns the authenticated caller, if any.
func CurrentPrincipal(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	return v.(Principal), true
}

// mustPrincipal is for handlers behind RequireUser.
func mustPrincipal(c *gin.Context) Principal {
	p, ok := CurrentPrincipal(c)
	if !ok {
		panic("identity: handler not behind RequireUser")
	}
	return p
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// Register mounts the identity routes on the /api/v1 group, which must run
// Authenticate.
func (h *HTTP) Register(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", h.login)
	auth.POST("/logout", RequireUser(), RequireSession(), h.logout)
	auth.POST("/password/forgot", h.forgotPassword)
	auth.POST("/password/reset", h.resetPassword)

	me := api.Group("/me", RequireUser())
	me.GET("", h.me)
	me.PATCH("", h.updateMe)

	credentials := me.Group("", RequireSession())
	credentials.POST("/password", h.changePassword)
	credentials.GET("/sessions", h.sessions)
	credentials.POST("/sessions/revoke-others", h.revokeOtherSessions)
	credentials.DELETE("/sessions/:id", h.revokeSession)
	credentials.GET("/tokens", h.tokens)
	credentials.POST("/tokens", h.createToken)
	credentials.DELETE("/tokens/:id", h.revokeToken)
}

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" example:"ada@example.com"`
	Password string `json:"password"`
}

// MeResponse describes the signed-in user. CSRFToken goes in X-CSRF-Token on
// state-changing requests.
type MeResponse struct {
	User      User   `json:"user"`
	CSRFToken string `json:"csrf_token"`
}

func client(c *gin.Context) Client {
	ip, _ := netip.ParseAddr(c.ClientIP())
	return Client{UserAgent: c.Request.UserAgent(), IP: ip}
}

// login opens a session.
//
//	@Summary		Sign in with email and password
//	@Description	Sets the session cookie. Failures are throttled per email and per IP; every failure answers invalid_credentials.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	MeResponse
//	@Failure		401		{object}	server.Problem	"invalid_credentials"
//	@Failure		429		{object}	server.Problem	"too_many_attempts, with Retry-After"
//	@Router			/api/v1/auth/login [post]
func (h *HTTP) login(c *gin.Context) {
	var req LoginRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	in, err := h.svc.Login(c, req.Email, req.Password, client(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	h.setCookie(c, in.Token, int(SessionIdleTimeout.Seconds()))
	c.JSON(http.StatusOK, MeResponse{User: in.User, CSRFToken: in.CSRF})
}

// logout revokes the current session.
//
//	@Summary	Sign out
//	@Tags		auth
//	@Success	204
//	@Failure	401	{object}	server.Problem
//	@Router		/api/v1/auth/logout [post]
func (h *HTTP) logout(c *gin.Context) {
	if err := h.svc.Logout(c, mustPrincipal(c)); err != nil {
		server.WriteError(c, err)
		return
	}
	h.clearCookie(c)
	c.Status(http.StatusNoContent)
}

// ForgotPasswordRequest is the body of POST /auth/password/forgot.
type ForgotPasswordRequest struct {
	Email string `json:"email" example:"ada@example.com"`
}

// forgotPassword mails a reset link.
//
//	@Summary		Request a password reset link
//	@Description	Answers 202 whether or not the email belongs to an account.
//	@Tags			auth
//	@Accept			json
//	@Param			body	body	ForgotPasswordRequest	true	"Account email"
//	@Success		202
//	@Failure		422	{object}	server.Problem
//	@Router			/api/v1/auth/password/forgot [post]
func (h *HTTP) forgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.RequestPasswordReset(c, req.Email); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// ResetPasswordRequest is the body of POST /auth/password/reset.
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// resetPassword sets a new password with a reset token.
//
//	@Summary		Reset a password
//	@Description	Signs out every session of the account.
//	@Tags			auth
//	@Accept			json
//	@Param			body	body	ResetPasswordRequest	true	"Token from the emailed link and the new password"
//	@Success		204
//	@Failure		422	{object}	server.Problem	"token invalid_or_expired, password too_short"
//	@Router			/api/v1/auth/password/reset [post]
func (h *HTTP) resetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.ResetPassword(c, req.Token, req.Password); err != nil {
		server.WriteError(c, err)
		return
	}
	h.clearCookie(c)
	c.Status(http.StatusNoContent)
}

// me returns the signed-in user and the CSRF token.
//
//	@Summary	Current user
//	@Tags		me
//	@Produce	json
//	@Success	200	{object}	MeResponse
//	@Header		200	{string}	ETag	"user version"
//	@Failure	401	{object}	server.Problem
//	@Router		/api/v1/me [get]
func (h *HTTP) me(c *gin.Context) {
	p := mustPrincipal(c)
	server.SetETag(c, p.User.Version)
	c.JSON(http.StatusOK, MeResponse{User: p.User, CSRFToken: h.svc.CSRFToken(p)})
}

// UpdateMeRequest changes the fields that are present.
type UpdateMeRequest struct {
	Name     *string `json:"name,omitempty"`
	Locale   *string `json:"locale,omitempty" example:"en"`
	Timezone *string `json:"timezone,omitempty" example:"Europe/Paris"`
}

// updateMe edits the profile.
//
//	@Summary		Update the current user's profile
//	@Description	An empty timezone follows the browser's.
//	@Tags			me
//	@Accept			json
//	@Produce		json
//	@Param			If-Match	header		string			false	"ETag from GET /me"
//	@Param			body		body		UpdateMeRequest	true	"Fields to change"
//	@Success		200			{object}	User
//	@Failure		409			{object}	server.Problem	"version_conflict"
//	@Failure		422			{object}	server.Problem
//	@Router			/api/v1/me [patch]
func (h *HTTP) updateMe(c *gin.Context) {
	var req UpdateMeRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	version, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	u, err := h.svc.UpdateProfile(c, mustPrincipal(c).User.ID, version, ProfileUpdate(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	server.SetETag(c, u.Version)
	c.JSON(http.StatusOK, u)
}

// ChangePasswordRequest is the body of POST /me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// changePassword replaces the password.
//
//	@Summary		Change the current user's password
//	@Description	Signs out the user's other sessions.
//	@Tags			me
//	@Accept			json
//	@Param			body	body	ChangePasswordRequest	true	"Current and new password"
//	@Success		204
//	@Failure		422	{object}	server.Problem	"current_password incorrect, new_password too_short"
//	@Router			/api/v1/me/password [post]
func (h *HTTP) changePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	if err := h.svc.ChangePassword(c, mustPrincipal(c), req.CurrentPassword, req.NewPassword); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SessionList is the body of GET /me/sessions.
type SessionList struct {
	Items []Session `json:"items"`
}

// sessions lists the user's sessions.
//
//	@Summary	List the current user's sessions
//	@Tags		me
//	@Produce	json
//	@Success	200	{object}	SessionList
//	@Router		/api/v1/me/sessions [get]
func (h *HTTP) sessions(c *gin.Context) {
	list, err := h.svc.Sessions(c, mustPrincipal(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, SessionList{Items: list})
}

// RevokedResponse counts revoked sessions.
type RevokedResponse struct {
	Revoked int64 `json:"revoked"`
}

// revokeOtherSessions signs out everywhere else.
//
//	@Summary	Sign out every other session
//	@Tags		me
//	@Produce	json
//	@Success	200	{object}	RevokedResponse
//	@Router		/api/v1/me/sessions/revoke-others [post]
func (h *HTTP) revokeOtherSessions(c *gin.Context) {
	n, err := h.svc.RevokeOtherSessions(c, mustPrincipal(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, RevokedResponse{Revoked: n})
}

// revokeSession signs one session out.
//
//	@Summary	Sign out one session
//	@Tags		me
//	@Param		id	path	string	true	"Session id"
//	@Success	204
//	@Failure	404	{object}	server.Problem	"session_not_found"
//	@Router		/api/v1/me/sessions/{id} [delete]
func (h *HTTP) revokeSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("session_not_found"))
		return
	}
	p := mustPrincipal(c)
	if err := h.svc.RevokeSession(c, p, id); err != nil {
		server.WriteError(c, err)
		return
	}
	if id == p.SessionID {
		h.clearCookie(c)
	}
	c.Status(http.StatusNoContent)
}

// TokenList is the body of GET /me/tokens.
type TokenList struct {
	Items []APIToken `json:"items"`
}

// tokens lists the user's API tokens.
//
//	@Summary		List the current user's API tokens
//	@Description	Needs a browser session, not a token.
//	@Tags			me
//	@Produce		json
//	@Success		200	{object}	TokenList
//	@Failure		403	{object}	server.Problem	"session_required"
//	@Router			/api/v1/me/tokens [get]
func (h *HTTP) tokens(c *gin.Context) {
	list, err := h.svc.APITokens(c, mustPrincipal(c))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, TokenList{Items: list})
}

// CreateTokenRequest is the body of POST /me/tokens.
type CreateTokenRequest struct {
	Name      string     `json:"name" example:"CI pipeline"`
	Scope     Scope      `json:"scope" enums:"read,write,admin"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// createToken issues an API token.
//
//	@Summary		Create an API token
//	@Description	The response is the only time the token is shown. Needs a browser session.
//	@Tags			me
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateTokenRequest	true	"Name, scope and optional expiry"
//	@Success		201		{object}	CreatedAPIToken
//	@Failure		409		{object}	server.Problem	"too_many_tokens"
//	@Failure		422		{object}	server.Problem
//	@Router			/api/v1/me/tokens [post]
func (h *HTTP) createToken(c *gin.Context) {
	var req CreateTokenRequest
	if err := server.BindJSON(c, &req); err != nil {
		server.WriteError(c, err)
		return
	}
	t, err := h.svc.CreateAPIToken(c, mustPrincipal(c), NewAPIToken(req))
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

// revokeToken deletes an API token.
//
//	@Summary	Revoke an API token
//	@Tags		me
//	@Param		id	path	string	true	"Token id"
//	@Success	204
//	@Failure	404	{object}	server.Problem	"token_not_found"
//	@Router		/api/v1/me/tokens/{id} [delete]
func (h *HTTP) revokeToken(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound("token_not_found"))
		return
	}
	if err := h.svc.RevokeAPIToken(c, mustPrincipal(c), id); err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
