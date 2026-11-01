package search

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves GET /w/{ws}/search.
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// Register mounts the route on the /w/:ws group.
func (h *HTTP) Register(ws *gin.RouterGroup) {
	ws.GET("/search", h.search)
}

// search finds issues.
//
//	@Summary		Search issues
//	@Description	By key (former keys too), words in titles, descriptions and comments (each word a prefix), and fuzzy titles; in the projects whose issues the caller sees. Snippets mark matched words with U+0002 and U+0003.
//	@Tags			search
//	@Produce		json
//	@Param			ws			path		string	true	"Workspace slug"
//	@Param			q			query		string	true	"What to look for"
//	@Param			limit		query		int		false	"At most 100; 20 by default"
//	@Param			archived	query		bool	false	"Include archived issues"
//	@Param			project		query		string	false	"Only this project (key)"
//	@Success		200			{object}	Results
//	@Router			/api/v1/w/{ws}/search [get]
func (h *HTTP) search(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	r, err := h.svc.Search(c, workspace.Current(c), c.Query("q"), Options{Limit: limit, Archived: c.Query("archived") == "true", Project: c.Query("project")})
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}
