package query_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/query"
)

func ctx() query.Context {
	return query.Context{Me: uuid.New(), Today: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), Projects: map[string]uuid.UUID{"HZN": uuid.New()}}
}

func compile(t *testing.T, filter string) (query.Compiled, error) {
	t.Helper()
	var n query.Node
	if err := json.Unmarshal([]byte(filter), &n); err != nil {
		t.Fatal(err)
	}
	return query.Compile(query.Query{Filter: &n}, ctx(), 3)
}

func TestValuesAreParameters(t *testing.T) {
	c, err := compile(t, `{"and":[{"field":"title","op":"contains","value":"'; DROP TABLE issues; --"},{"field":"project","op":"in","value":["HZN"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Where, "DROP") || !strings.Contains(c.Where, "$3") || !strings.Contains(c.Where, "$4") || len(c.Args) != 2 {
		t.Errorf("where = %s, args %v", c.Where, c.Args)
	}
	if !strings.HasSuffix(c.Where, "AND i.archived_at IS NULL") {
		t.Errorf("archived not excluded: %s", c.Where)
	}
}

func TestRelativeDates(t *testing.T) {
	c, err := compile(t, `{"field":"due_date","op":"within","value":"+7d"}`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Args[0].(time.Time).Format(time.DateOnly) != "2026-11-04" || c.Args[1].(time.Time).Format(time.DateOnly) != "2026-11-11" {
		t.Errorf("args = %v", c.Args)
	}
}

func TestErrors(t *testing.T) {
	for filter, code := range map[string]string{
		`{"field":"nope","op":"eq","value":"x"}`:                      "unknown_field",
		`{"field":"priority","op":"eq","value":"asap"}`:               "bad_value",
		`{"field":"due_date","op":"contains","value":"x"}`:            "bad_value",
		`{"field":"archived","op":"lt","value":true}`:                 "bad_op",
		`{"field":"cf:` + uuid.NewString() + `","op":"eq","value":1}`: "unknown_field",
		`{}`: "bad_value",
	} {
		_, err := compile(t, filter)
		var qe *query.Error
		if err == nil || !strings.Contains(err.Error(), code) {
			t.Errorf("%s: %v, want %s (%v)", filter, err, code, qe)
		}
	}
	deep := `{"field":"title","op":"contains","value":"x"}`
	for range 8 {
		deep = `{"not":` + deep + `}`
	}
	if _, err := compile(t, deep); err == nil || !strings.Contains(err.Error(), "too_complex") {
		t.Errorf("deep: %v", err)
	}
	if _, err := query.Compile(query.Query{Sort: []query.Sort{{Field: "drop table"}}}, ctx(), 1); err == nil {
		t.Error("unknown sort accepted")
	}
}
