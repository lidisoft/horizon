// Package document is Horizon's block document model (docs/realtime.md,
// ADR-0015): a JSON tree of nodes checked against a schema, positions as
// offsets into its flattened token sequence, and steps that change it and
// map positions through each other. web/app/editor implements the same in
// JavaScript; both run the vectors in testdata/, so they can't drift.
//
// The algorithms follow ProseMirror's model and transform modules, which
// the editor's protocol is modelled on.
package document

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

//go:embed schema.json
var schemaJSON []byte

// SchemaJSON is the schema's source, for the JavaScript module.
func SchemaJSON() []byte { return schemaJSON }

// Schema is the set of node and mark types documents may use.
type Schema struct {
	Nodes    map[string]*NodeType
	Marks    map[string]*MarkType
	Top      *NodeType
	MaxBytes int
	MaxDepth int
	MaxText  int // longest string attribute
}

// NodeType is a kind of node.
type NodeType struct {
	Name    string
	Groups  []string
	content []term
	expr    string
	marks   map[string]bool // nil: every mark
	Code    bool
	Attrs   map[string]AttrSpec
	attrSeq []string // sorted names
	schema  *Schema
}

// MarkType is a kind of mark.
type MarkType struct {
	Name  string
	Rank  int
	Attrs map[string]AttrSpec
}

// AttrSpec describes one attribute: its default (none: required) and the
// values it takes.
type AttrSpec struct {
	Default    any   `json:"default"`
	HasDefault bool  `json:"-"`
	Values     []any `json:"values"`
	Kind       string
}

// term is one part of a content expression: names (types or groups) and
// how many times they may repeat.
type term struct {
	names    []string
	min, max int // max < 0: unbounded
}

type schemaSpec struct {
	Top   string `json:"top"`
	Nodes map[string]struct {
		Group   string                     `json:"group"`
		Content string                     `json:"content"`
		Marks   *[]string                  `json:"marks"`
		Code    bool                       `json:"code"`
		Attrs   map[string]json.RawMessage `json:"attrs"`
	} `json:"nodes"`
	Marks []struct {
		Name  string                     `json:"name"`
		Attrs map[string]json.RawMessage `json:"attrs"`
	} `json:"marks"`
	Limits struct {
		MaxBytes  int `json:"max_bytes"`
		MaxDepth  int `json:"max_depth"`
		MaxString int `json:"max_string"`
	} `json:"limits"`
}

var defaultSchema = mustParse(schemaJSON)

// Default returns Horizon's schema (schema.json).
func Default() *Schema { return defaultSchema }

func mustParse(raw []byte) *Schema {
	s, err := ParseSchema(raw)
	if err != nil {
		panic(err)
	}
	return s
}

// ParseSchema reads a schema in the format of schema.json.
func ParseSchema(raw []byte) (*Schema, error) {
	var spec schemaSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("document: schema: %w", err)
	}
	s := &Schema{Nodes: map[string]*NodeType{}, Marks: map[string]*MarkType{},
		MaxBytes: spec.Limits.MaxBytes, MaxDepth: spec.Limits.MaxDepth, MaxText: spec.Limits.MaxString}
	for i, m := range spec.Marks {
		attrs, err := parseAttrs(m.Attrs)
		if err != nil {
			return nil, fmt.Errorf("document: mark %s: %w", m.Name, err)
		}
		s.Marks[m.Name] = &MarkType{Name: m.Name, Rank: i, Attrs: attrs}
	}
	for name, n := range spec.Nodes {
		t := &NodeType{Name: name, Code: n.Code, expr: n.Content, schema: s}
		if n.Group != "" {
			t.Groups = strings.Fields(n.Group)
		}
		if n.Marks != nil {
			t.marks = map[string]bool{}
			for _, m := range *n.Marks {
				t.marks[m] = true
			}
		}
		attrs, err := parseAttrs(n.Attrs)
		if err != nil {
			return nil, fmt.Errorf("document: node %s: %w", name, err)
		}
		t.Attrs = attrs
		for a := range attrs {
			t.attrSeq = append(t.attrSeq, a)
		}
		slices.Sort(t.attrSeq)
		s.Nodes[name] = t
	}
	for _, t := range s.Nodes {
		terms, err := parseContent(t.expr, s)
		if err != nil {
			return nil, fmt.Errorf("document: node %s: %w", t.Name, err)
		}
		t.content = terms
	}
	s.Top = s.Nodes[spec.Top]
	if s.Top == nil || s.Nodes["text"] == nil {
		return nil, fmt.Errorf("document: schema needs %q and text nodes", spec.Top)
	}
	return s, nil
}

func parseAttrs(raw map[string]json.RawMessage) (map[string]AttrSpec, error) {
	out := map[string]AttrSpec{}
	for name, r := range raw {
		var spec AttrSpec
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(r, &probe); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(r, &spec); err != nil {
			return nil, err
		}
		if k, ok := probe["kind"]; ok {
			_ = json.Unmarshal(k, &spec.Kind)
		}
		_, spec.HasDefault = probe["default"]
		spec.Default = normalize(spec.Default)
		for i, v := range spec.Values {
			spec.Values[i] = normalize(v)
		}
		out[name] = spec
	}
	return out, nil
}

var quantified = regexp.MustCompile(`^([a-z_]+)([?*+]?)$`)

func parseContent(expr string, s *Schema) ([]term, error) {
	var out []term
	for _, part := range strings.Fields(expr) {
		m := quantified.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("content %q: bad term %q", expr, part)
		}
		t := term{min: 1, max: 1}
		switch m[2] {
		case "?":
			t.min = 0
		case "*":
			t.min, t.max = 0, -1
		case "+":
			t.max = -1
		}
		if _, ok := s.Nodes[m[1]]; ok {
			t.names = []string{m[1]}
		} else {
			for name, nt := range s.Nodes {
				for _, g := range nt.Groups {
					if g == m[1] {
						t.names = append(t.names, name)
					}
				}
			}
			if len(t.names) == 0 {
				return nil, fmt.Errorf("content %q: unknown %q", expr, m[1])
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// IsText reports whether the type is the text type.
func (t *NodeType) IsText() bool { return t.Name == "text" }

// IsInline reports whether the type is in the inline group.
func (t *NodeType) IsInline() bool { return t.InGroup("inline") }

// IsLeaf reports whether nodes of the type have no content.
func (t *NodeType) IsLeaf() bool { return len(t.content) == 0 }

// IsTextblock reports whether the type holds inline content.
func (t *NodeType) IsTextblock() bool {
	for _, tm := range t.content {
		for _, n := range tm.names {
			if t.schema.Nodes[n].IsInline() {
				return true
			}
		}
	}
	return false
}

// InGroup reports whether the type belongs to group.
func (t *NodeType) InGroup(group string) bool {
	for _, g := range t.Groups {
		if g == group {
			return true
		}
	}
	return false
}

// AllowsMark reports whether children of nodes of this type may carry m.
func (t *NodeType) AllowsMark(m *MarkType) bool { return t.marks == nil || t.marks[m.Name] }

// compatibleContent reports whether the content of one type can be joined
// onto the other's: same type, or the same content expression.
func (t *NodeType) compatibleContent(o *NodeType) bool {
	return t == o || (t.expr != "" && t.expr == o.expr)
}

// ValidContent reports whether a fragment may be the content of a node of
// this type.
func (t *NodeType) ValidContent(f Fragment) bool {
	i := 0
	for _, tm := range t.content {
		n := 0
		for i < len(f.nodes) && (tm.max < 0 || n < tm.max) && contains(tm.names, f.nodes[i].Type.Name) {
			i++
			n++
		}
		if n < tm.min {
			return false
		}
	}
	if i != len(f.nodes) {
		return false
	}
	for _, c := range f.nodes {
		for _, m := range c.Marks {
			if !t.AllowsMark(m.Type) {
				return false
			}
		}
	}
	return true
}

func (t *NodeType) checkContent(f Fragment) error {
	if !t.ValidContent(f) {
		return &Error{Code: "invalid_content", Detail: fmt.Sprintf("invalid content for node %s", t.Name)}
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// normalize turns JSON numbers into float64 and leaves the rest alone, so
// attribute values compare the same however they were built.
func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return v
}

var issueKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$`)

// computeAttrs fills defaults and checks every value.
func computeAttrs(specs map[string]AttrSpec, given map[string]any, maxText int, what string) (map[string]any, error) {
	if len(specs) == 0 {
		if len(given) > 0 {
			return nil, &Error{Code: "invalid_attrs", Detail: what + " takes no attributes"}
		}
		return nil, nil
	}
	out := make(map[string]any, len(specs))
	for name, spec := range specs {
		v, ok := given[name]
		if !ok || v == nil {
			if !spec.HasDefault {
				return nil, &Error{Code: "invalid_attrs", Detail: fmt.Sprintf("%s needs attribute %s", what, name)}
			}
			v = spec.Default
		}
		v = normalize(v)
		if err := checkAttr(spec, v, maxText); err != nil {
			return nil, &Error{Code: "invalid_attrs", Detail: fmt.Sprintf("%s attribute %s: %v", what, name, err)}
		}
		out[name] = v
	}
	for name := range given {
		if _, ok := specs[name]; !ok {
			return nil, &Error{Code: "invalid_attrs", Detail: fmt.Sprintf("%s has no attribute %s", what, name)}
		}
	}
	return out, nil
}

func checkAttr(spec AttrSpec, v any, maxText int) error {
	if len(spec.Values) > 0 {
		for _, ok := range spec.Values {
			if ok == v {
				return nil
			}
		}
		return fmt.Errorf("value %v not allowed", v)
	}
	switch spec.Kind {
	case "count":
		f, ok := v.(float64)
		if !ok || f < 1 || f != float64(int64(f)) || f > 1e6 {
			return fmt.Errorf("want a whole number ≥ 1")
		}
	case "string", "uuid", "issue_key", "url":
		s, ok := v.(string)
		if !ok || !utf8.ValidString(s) || len([]rune(s)) > maxText {
			return fmt.Errorf("want a string of at most %d characters", maxText)
		}
		switch spec.Kind {
		case "uuid":
			if _, err := uuid.Parse(s); err != nil {
				return fmt.Errorf("want an id")
			}
		case "issue_key":
			if !issueKeyRe.MatchString(s) {
				return fmt.Errorf("want an issue key")
			}
		case "url":
			u, err := url.Parse(s)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto") {
				return fmt.Errorf("want an http, https or mailto link")
			}
		}
	}
	return nil
}

// Error is a document or step that doesn't hold.
type Error struct {
	Code   string // invalid_document, invalid_content, invalid_attrs, invalid_step, too_large
	Detail string
}

func (e *Error) Error() string { return "document: " + e.Detail }

func errorf(code, format string, args ...any) error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}
