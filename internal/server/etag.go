package server

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lidisoft/horizon/internal/apperr"
)

// Optimistic concurrency (docs/api.md#conventions): responses carry the
// aggregate version as a strong ETag, mutations may send it back in If-Match.

// SetETag sets ETag: "<version>".
func SetETag(c *gin.Context, version int32) {
	c.Header("ETag", strconv.Quote(strconv.FormatInt(int64(version), 10)))
}

// IfMatch returns the version in If-Match, or 0 when the header is absent.
func IfMatch(c *gin.Context) (int32, error) {
	h := strings.TrimSpace(c.GetHeader("If-Match"))
	if h == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(h, "W/"), `"`), 10, 32)
	if err != nil || v < 1 {
		return 0, apperr.BadRequest(`If-Match: want an ETag such as "3"`)
	}
	return int32(v), nil
}
