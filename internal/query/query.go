// Package query compiles the issue query language (docs/api.md#querying-issues)
// to SQL: a tree of conditions on built-in and custom fields, placeholders
// (@me, @today, @current_iteration), sorting. It produces a WHERE and an
// ORDER BY over issues aliased i, with their status s and category sc
// joined; every value is a bind parameter.
package query

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Limits of a query.
const (
	MaxDepth      = 6
	MaxConditions = 50
	MaxValues     = 200
	DefaultLimit  = 100
	MaxLimit      = 500
)

// Query is what a saved view stores and POST …/issues/query takes.
type Query struct {
	Filter  *Node  `json:"filter,omitempty"`
	GroupBy string `json:"group_by,omitempty" example:"status"`
	Sort    []Sort `json:"sort,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	// Archived includes archived issues.
	Archived bool `json:"archived,omitempty"`
}

// Node is a condition, or a group of them (and, or, not).
type Node struct {
	And   []*Node         `json:"and,omitempty"`
	Or    []*Node         `json:"or,omitempty"`
	Not   *Node           `json:"not,omitempty"`
	Field string          `json:"field,omitempty" example:"status.outcome"`
	Op    string          `json:"op,omitempty" example:"eq"`
	Value json.RawMessage `json:"value,omitempty" swaggertype:"object"`
}

// Sort is one sort key.
type Sort struct {
	Field string `json:"field" example:"rank"`
	Dir   string `json:"dir,omitempty" enums:"asc,desc"`
}

// Context is what values and placeholders resolve against.
type Context struct {
	Me                uuid.UUID
	Today             time.Time            // the workspace's date
	CurrentIterations []uuid.UUID          // active iterations of the projects queried
	Projects          map[string]uuid.UUID // visible projects by key (current and former)
	// CustomFields gives a custom field's type (text, number, select,
	// multi_select, date, user, url, checkbox); ok false when unknown.
	CustomFields func(id uuid.UUID) (string, bool)
	// IssueID resolves an issue key the caller sees (parent conditions).
	IssueID func(key string) (uuid.UUID, bool)
}

// Error is a query that doesn't compile, with the place it went wrong.
type Error struct {
	Path string // e.g. filter.and[1].value
	Code string // unknown_field, bad_op, bad_value, too_complex, bad_sort
}

func (e *Error) Error() string { return fmt.Sprintf("query: %s: %s", e.Path, e.Code) }

// Compiled is SQL ready to embed: WHERE and ORDER BY clauses whose
// parameters are numbered from the first given to Compile.
type Compiled struct {
	Where   string
	OrderBy string
	Args    []any
}

type compiler struct {
	ctx        Context
	args       []any
	first      int
	conditions int
}

func (c *compiler) arg(v any) string {
	c.args = append(c.args, v)
	return "$" + strconv.Itoa(c.first+len(c.args)-1)
}

// Compile turns q into SQL; firstArg is the number of the first parameter.
func Compile(q Query, ctx Context, firstArg int) (Compiled, error) {
	c := &compiler{ctx: ctx, first: firstArg}
	where := "TRUE"
	if q.Filter != nil {
		w, err := c.node(q.Filter, "filter", 0)
		if err != nil {
			return Compiled{}, err
		}
		where = w
	}
	if !q.Archived {
		where = "(" + where + ") AND i.archived_at IS NULL"
	}
	order, err := c.order(q.Sort)
	if err != nil {
		return Compiled{}, err
	}
	return Compiled{Where: where, OrderBy: order, Args: c.args}, nil
}

func (c *compiler) node(n *Node, path string, depth int) (string, error) {
	if depth > MaxDepth {
		return "", &Error{Path: path, Code: "too_complex"}
	}
	switch {
	case len(n.And) > 0 || len(n.Or) > 0:
		list, op, key := n.And, " AND ", "and"
		if len(n.Or) > 0 {
			list, op, key = n.Or, " OR ", "or"
		}
		parts := make([]string, 0, len(list))
		for i, child := range list {
			if child == nil {
				return "", &Error{Path: fmt.Sprintf("%s.%s[%d]", path, key, i), Code: "bad_value"}
			}
			p, err := c.node(child, fmt.Sprintf("%s.%s[%d]", path, key, i), depth+1)
			if err != nil {
				return "", err
			}
			parts = append(parts, "("+p+")")
		}
		return strings.Join(parts, op), nil
	case n.Not != nil:
		p, err := c.node(n.Not, path+".not", depth+1)
		if err != nil {
			return "", err
		}
		return "NOT (" + p + ")", nil
	case n.Field != "":
		c.conditions++
		if c.conditions > MaxConditions {
			return "", &Error{Path: path, Code: "too_complex"}
		}
		return c.condition(n, path)
	}
	return "", &Error{Path: path, Code: "bad_value"}
}

// kind is how a field compares.
type kind int

const (
	kText kind = iota
	kNumber
	kDate
	kTime
	kID
	kBool
	kSet // a set of ids (labels, multi_select)
)

// fieldDef is a built-in field: its SQL and kind.
type fieldDef struct {
	sql  string
	kind kind
	ids  func(c *compiler, raw string) (any, bool) // id-like values: resolve one
}

var priorities = []string{"none", "urgent", "high", "medium", "low"}

func (c *compiler) builtin(name string) (fieldDef, bool) {
	id := func(c *compiler, raw string) (any, bool) {
		u, err := uuid.Parse(raw)
		return u, err == nil
	}
	user := func(c *compiler, raw string) (any, bool) {
		if raw == "@me" {
			return c.ctx.Me, true
		}
		return id(c, raw)
	}
	switch name {
	case "project":
		return fieldDef{sql: "i.project_id", kind: kID, ids: func(c *compiler, raw string) (any, bool) {
			if p, ok := c.ctx.Projects[strings.ToUpper(raw)]; ok {
				return p, true
			}
			return id(c, raw)
		}}, true
	case "type":
		return fieldDef{sql: "i.type_id", kind: kID, ids: id}, true
	case "status":
		return fieldDef{sql: "i.status_id", kind: kID, ids: id}, true
	case "status.category":
		return fieldDef{sql: "s.category_id", kind: kID, ids: id}, true
	case "status.outcome":
		return fieldDef{sql: "sc.outcome", kind: kText}, true
	case "priority":
		return fieldDef{sql: "i.priority", kind: kText}, true
	case "assignee":
		return fieldDef{sql: "i.assignee_id", kind: kID, ids: user}, true
	case "reporter":
		return fieldDef{sql: "i.reporter_id", kind: kID, ids: user}, true
	case "parent":
		return fieldDef{sql: "i.parent_id", kind: kID, ids: func(c *compiler, raw string) (any, bool) {
			if c.ctx.IssueID != nil {
				if u, ok := c.ctx.IssueID(raw); ok {
					return u, true
				}
			}
			return id(c, raw)
		}}, true
	case "iteration":
		return fieldDef{sql: "i.iteration_id", kind: kID, ids: id}, true
	case "release":
		return fieldDef{sql: "i.release_id", kind: kID, ids: id}, true
	case "labels":
		return fieldDef{sql: "labels", kind: kSet, ids: id}, true
	case "estimate":
		return fieldDef{sql: "i.estimate", kind: kText}, true
	case "title":
		return fieldDef{sql: "i.title", kind: kText}, true
	case "text":
		return fieldDef{sql: "text", kind: kText}, true
	case "start_date":
		return fieldDef{sql: "i.start_date", kind: kDate}, true
	case "due_date":
		return fieldDef{sql: "i.due_date", kind: kDate}, true
	case "created":
		return fieldDef{sql: "i.created_at", kind: kTime}, true
	case "updated":
		return fieldDef{sql: "i.updated_at", kind: kTime}, true
	case "resolved":
		return fieldDef{sql: "i.resolved_at", kind: kTime}, true
	case "started":
		return fieldDef{sql: "i.started_at", kind: kTime}, true
	case "archived":
		return fieldDef{sql: "(i.archived_at IS NOT NULL)", kind: kBool}, true
	}
	return fieldDef{}, false
}

// custom resolves cf:<id>.
func (c *compiler) custom(name string) (fieldDef, bool) {
	id, err := uuid.Parse(strings.TrimPrefix(name, "cf:"))
	if err != nil || c.ctx.CustomFields == nil {
		return fieldDef{}, false
	}
	typ, ok := c.ctx.CustomFields(id)
	if !ok {
		return fieldDef{}, false
	}
	path := c.arg(id.String())
	text := "(i.fields->>" + path + ")"
	switch typ {
	case "number":
		return fieldDef{sql: "(i.fields->>" + path + ")::numeric", kind: kNumber}, true
	case "date":
		return fieldDef{sql: "(i.fields->>" + path + ")::date", kind: kDate}, true
	case "checkbox":
		return fieldDef{sql: "coalesce((i.fields->>" + path + ")::boolean, false)", kind: kBool}, true
	case "multi_select":
		return fieldDef{sql: "cf:" + path, kind: kSet}, true
	case "user":
		return fieldDef{sql: text, kind: kText}, true
	}
	return fieldDef{sql: text, kind: kText}, true
}

var relative = regexp.MustCompile(`^([+-]?\d{1,4})([dwm])$`)

// dateValue reads a date: YYYY-MM-DD, @today, or relative to today (-7d, +2w, 1m).
func (c *compiler) dateValue(raw json.RawMessage) (time.Time, bool) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return time.Time{}, false
	}
	if s == "@today" {
		return c.ctx.Today, true
	}
	if m := relative.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "d":
			return c.ctx.Today.AddDate(0, 0, n), true
		case "w":
			return c.ctx.Today.AddDate(0, 0, 7*n), true
		default:
			return c.ctx.Today.AddDate(0, n, 0), true
		}
	}
	t, err := time.Parse(time.DateOnly, s)
	return t, err == nil
}

// values reads a value or a list of them as strings (ids, texts).
func values(raw json.RawMessage) ([]string, bool) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, true
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil && len(many) <= MaxValues {
		return many, true
	}
	return nil, false
}

func (c *compiler) condition(n *Node, path string) (string, error) {
	n = &Node{Field: n.Field, Op: n.Op, Value: n.Value} // values get rewritten: leave the caller's alone
	var f fieldDef
	var ok bool
	if strings.HasPrefix(n.Field, "cf:") {
		f, ok = c.custom(n.Field)
	} else {
		f, ok = c.builtin(n.Field)
	}
	if !ok {
		return "", &Error{Path: path + ".field", Code: "unknown_field"}
	}
	bad := &Error{Path: path + ".value", Code: "bad_value"}
	badOp := &Error{Path: path + ".op", Code: "bad_op"}
	op := n.Op

	// Presence works for every field.
	if op == "is_empty" || op == "is_not_empty" {
		var sql string
		switch {
		case f.sql == "labels":
			sql = "NOT EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = i.id)"
		case strings.HasPrefix(f.sql, "cf:"):
			key := strings.TrimPrefix(f.sql, "cf:")
			sql = "coalesce(jsonb_array_length(i.fields->" + key + "), 0) = 0"
		case f.sql == "text":
			return "", badOp
		case f.kind == kBool:
			return "", badOp
		case f.kind == kText:
			sql = "coalesce(" + f.sql + ", '') = ''"
		default:
			sql = f.sql + " IS NULL"
		}
		if op == "is_not_empty" {
			sql = "NOT (" + sql + ")"
		}
		return sql, nil
	}

	switch {
	case f.sql == "text":
		// Words anywhere in the title or description, as search does.
		var s string
		if op != "contains" || json.Unmarshal(n.Value, &s) != nil || strings.TrimSpace(s) == "" {
			return "", bad
		}
		return "(setweight(to_tsvector('simple', i.title), 'A') || setweight(to_tsvector('simple', i.description), 'B')) @@ to_tsquery('simple', " +
			c.arg(prefixQuery(s)) + ")", nil

	case f.kind == kSet:
		vs, ok := values(n.Value)
		if !ok || len(vs) == 0 {
			return "", bad
		}
		var test func(list string) string
		if f.sql == "labels" {
			ids := make([]uuid.UUID, 0, len(vs))
			for _, v := range vs {
				u, ok := f.ids(c, v)
				if !ok {
					return "", bad
				}
				ids = append(ids, u.(uuid.UUID))
			}
			list := c.arg(ids)
			test = func(string) string {
				return "EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = i.id AND il.label_id = ANY(" + list + "::uuid[]))"
			}
			all := "(SELECT count(DISTINCT il.label_id) FROM issue_labels il WHERE il.issue_id = i.id AND il.label_id = ANY(" + list + "::uuid[])) = " + strconv.Itoa(len(ids))
			switch op {
			case "eq", "contains", "in":
				if op == "contains" && len(ids) > 1 {
					return all, nil
				}
				return test(""), nil
			case "neq", "nin":
				return "NOT " + test(""), nil
			}
			return "", badOp
		}
		key := strings.TrimPrefix(f.sql, "cf:")
		list := c.arg(vs)
		switch op {
		case "eq", "in":
			return "(i.fields->" + key + ") ?| " + list + "::text[]", nil
		case "contains":
			return "(i.fields->" + key + ") ?& " + list + "::text[]", nil
		case "neq", "nin":
			return "NOT coalesce((i.fields->" + key + ") ?| " + list + "::text[], false)", nil
		}
		return "", badOp

	case f.kind == kID:
		vs, ok := values(n.Value)
		if !ok || len(vs) == 0 {
			return "", bad
		}
		var ids []uuid.UUID
		for _, v := range vs {
			if v == "@current_iteration" && n.Field == "iteration" {
				ids = append(ids, c.ctx.CurrentIterations...)
				continue
			}
			u, ok := f.ids(c, v)
			if !ok {
				return "", bad
			}
			ids = append(ids, u.(uuid.UUID))
		}
		list := c.arg(ids)
		switch op {
		case "eq", "in":
			return f.sql + " = ANY(" + list + "::uuid[])", nil
		case "neq", "nin":
			return "(" + f.sql + " IS NULL OR NOT " + f.sql + " = ANY(" + list + "::uuid[]))", nil
		}
		return "", badOp

	case f.kind == kText:
		if n.Field == "status.outcome" {
			vs, ok := values(n.Value)
			if !ok {
				return "", bad
			}
			var out []string
			for _, v := range vs {
				switch v {
				case "open":
					out = append(out, "unstarted", "started")
				case "closed", "final":
					out = append(out, "completed", "abandoned")
				case "unstarted", "started", "completed", "abandoned":
					out = append(out, v)
				default:
					return "", bad
				}
			}
			n.Value, _ = json.Marshal(out)
			switch op {
			case "eq":
				op = "in"
			case "neq":
				op = "nin"
			}
		}
		if n.Field == "priority" {
			vs, _ := values(n.Value)
			for _, v := range vs {
				if !slices.Contains(priorities, v) {
					return "", bad
				}
			}
		}
		switch op {
		case "eq", "neq":
			var s string
			if json.Unmarshal(n.Value, &s) != nil {
				return "", bad
			}
			if op == "eq" {
				return f.sql + " = " + c.arg(s), nil
			}
			return "coalesce(" + f.sql + ", '') <> " + c.arg(s), nil
		case "in", "nin":
			vs, ok := values(n.Value)
			if !ok {
				return "", bad
			}
			if op == "in" {
				return f.sql + " = ANY(" + c.arg(vs) + "::text[])", nil
			}
			return "NOT coalesce(" + f.sql + " = ANY(" + c.arg(vs) + "::text[]), false)", nil
		case "contains":
			var s string
			if json.Unmarshal(n.Value, &s) != nil || s == "" {
				return "", bad
			}
			return f.sql + " ILIKE " + c.arg("%"+escapeLike(s)+"%") + " ESCAPE '\\'", nil
		}
		return "", badOp

	case f.kind == kNumber:
		var v float64
		if json.Unmarshal(n.Value, &v) != nil {
			return "", bad
		}
		cmp, ok := comparison(op)
		if !ok {
			return "", badOp
		}
		return f.sql + " " + cmp + " " + c.arg(v), nil

	case f.kind == kDate || f.kind == kTime:
		cast := "::date"
		if f.kind == kTime {
			cast = "::timestamptz"
		}
		if op == "within" {
			// Between today and the relative date (either side).
			d, ok := c.dateValue(n.Value)
			if !ok {
				return "", bad
			}
			from, to := c.ctx.Today, d
			if to.Before(from) {
				from, to = to, from
			}
			if f.kind == kTime {
				return f.sql + " >= " + c.arg(from) + "::timestamptz AND " + f.sql + " < " + c.arg(to.AddDate(0, 0, 1)) + "::timestamptz", nil
			}
			return f.sql + " BETWEEN " + c.arg(from) + "::date AND " + c.arg(to) + "::date", nil
		}
		d, ok := c.dateValue(n.Value)
		if !ok {
			return "", bad
		}
		cmp, ok := comparison(op)
		if !ok {
			return "", badOp
		}
		if f.kind == kTime {
			// Timestamps compare by the day they fall on.
			return "(" + f.sql + ")::date " + cmp + " " + c.arg(d) + "::date", nil
		}
		return f.sql + " " + cmp + " " + c.arg(d) + cast, nil

	case f.kind == kBool:
		var b bool
		if json.Unmarshal(n.Value, &b) != nil {
			return "", bad
		}
		switch op {
		case "eq":
			return f.sql + " = " + c.arg(b), nil
		case "neq":
			return f.sql + " <> " + c.arg(b), nil
		}
		return "", badOp
	}
	return "", badOp
}

func comparison(op string) (string, bool) {
	switch op {
	case "eq":
		return "=", true
	case "neq":
		return "<>", true
	case "lt":
		return "<", true
	case "lte":
		return "<=", true
	case "gt":
		return ">", true
	case "gte":
		return ">=", true
	}
	return "", false
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// prefixQuery makes "outbox rel" into outbox:* & rel:*.
func prefixQuery(s string) string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		words = append(words, w+":*")
		if len(words) == 12 {
			break
		}
	}
	if len(words) == 0 {
		return "''"
	}
	return strings.Join(words, " & ")
}

// sortable lists the fields issues sort by.
var sortable = map[string]string{
	"rank":       "i.rank",
	"key":        "i.number",
	"title":      "lower(i.title)",
	"created":    "i.created_at",
	"updated":    "i.updated_at",
	"resolved":   "i.resolved_at",
	"start_date": "i.start_date",
	"due_date":   "i.due_date",
	"priority":   "array_position(ARRAY['urgent','high','medium','low','none'], i.priority)",
	"status":     "sc.position * 1000 + s.position",
	"estimate":   "i.estimate",
}

func (c *compiler) order(sorts []Sort) (string, error) {
	if len(sorts) == 0 {
		return "i.rank, i.id", nil
	}
	if len(sorts) > 5 {
		return "", &Error{Path: "sort", Code: "too_complex"}
	}
	parts := make([]string, 0, len(sorts)+1)
	for k, s := range sorts {
		expr, ok := sortable[s.Field]
		if !ok && strings.HasPrefix(s.Field, "cf:") {
			if f, ok2 := c.custom(s.Field); ok2 && f.kind != kSet {
				expr, ok = f.sql, true
			}
		}
		if !ok {
			return "", &Error{Path: fmt.Sprintf("sort[%d].field", k), Code: "bad_sort"}
		}
		dir := "ASC"
		switch s.Dir {
		case "", "asc":
		case "desc":
			dir = "DESC"
		default:
			return "", &Error{Path: fmt.Sprintf("sort[%d].dir", k), Code: "bad_sort"}
		}
		parts = append(parts, expr+" "+dir+" NULLS LAST")
	}
	return strings.Join(append(parts, "i.id"), ", "), nil
}
