package report

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestFromQuery(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(t.Context(), "GET", `/?filter={"field":"priority","op":"eq","value":"high"}&projects=HZN,OPS&metric=estimate&last=5&iteration=01a19900-0000-7000-8000-000000000000&from=2026-10-01`, nil)
	r, err := requestFromQuery(c)
	if err != nil || r.Filter == nil || r.Filter.Field != "priority" || len(r.Projects) != 2 || r.Metric != "estimate" || r.Last != 5 ||
		!r.Iteration.Valid || r.From != "2026-10-01" {
		t.Errorf("request = %+v, %v", r, err)
	}
	c, _ = gin.CreateTestContext(httptest.NewRecorder()) // a context caches its query
	c.Request = httptest.NewRequestWithContext(t.Context(), "GET", `/?filter=nope&last=x&release=bad`, nil)
	if _, err := requestFromQuery(c); err == nil {
		t.Error("bad values accepted")
	}
}
