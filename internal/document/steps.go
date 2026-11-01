package document

import (
	"encoding/json"
	"maps"
	"reflect"
)

// Step is one change to a document (docs/realtime.md#steps). Steps apply to
// a document, invert against the document they applied to, and map through
// other changes (nil: the change they made is gone).
type Step interface {
	Apply(doc *Node) (*Node, error)
	Invert(doc *Node) (Step, error)
	Map(m Mapper) Step
	StepMap() StepMap
	json() map[string]any
}

// stepJSON reads a step's JSON form; stepType says which fields apply.
type stepJSON struct {
	StepType  string         `json:"stepType"`
	From      int            `json:"from"`
	To        int            `json:"to"`
	GapFrom   int            `json:"gapFrom"`
	GapTo     int            `json:"gapTo"`
	Insert    int            `json:"insert"`
	Slice     *sliceJSON     `json:"slice"`
	Structure bool           `json:"structure"`
	Mark      *markJSON      `json:"mark"`
	Pos       int            `json:"pos"`
	Attrs     map[string]any `json:"attrs"`
}

// MarshalStep writes a step as JSON (the same form as JavaScript's toJSON).
func MarshalStep(s Step) ([]byte, error) { return json.Marshal(s.json()) }

// withSlice adds the slice (when not empty) and the structure flag.
func withSlice(j map[string]any, sl Slice, structure bool) map[string]any {
	if sl.Content.size > 0 {
		j["slice"] = sl.toJSON()
	}
	if structure {
		j["structure"] = true
	}
	return j
}

// ParseStep reads a step and checks its slice and mark against the schema.
func (s *Schema) ParseStep(raw []byte) (Step, error) {
	var j stepJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, errorf("invalid_step", "%v", err)
	}
	slice := Slice{}
	if j.Slice != nil {
		var err error
		if slice, err = s.sliceFromJSON(*j.Slice); err != nil {
			return nil, err
		}
	}
	mark := func() (Mark, error) {
		if j.Mark == nil {
			return Mark{}, errorf("invalid_step", "%s needs a mark", j.StepType)
		}
		ms, err := s.marksFromJSON([]markJSON{*j.Mark})
		if err != nil {
			return Mark{}, err
		}
		return ms[0], nil
	}
	if j.From < 0 || j.To < j.From || j.Pos < 0 {
		return nil, errorf("invalid_step", "bad range %d–%d", j.From, j.To)
	}
	switch j.StepType {
	case "replace":
		return &ReplaceStep{From: j.From, To: j.To, Slice: slice, Structure: j.Structure}, nil
	case "replace_around":
		if j.GapFrom < j.From || j.GapTo < j.GapFrom || j.To < j.GapTo || j.Insert < 0 || j.Insert > slice.Size() {
			return nil, errorf("invalid_step", "bad gap")
		}
		return &ReplaceAroundStep{From: j.From, To: j.To, GapFrom: j.GapFrom, GapTo: j.GapTo, Slice: slice, Insert: j.Insert, Structure: j.Structure}, nil
	case "add_mark":
		m, err := mark()
		if err != nil {
			return nil, err
		}
		return &AddMarkStep{From: j.From, To: j.To, Mark: m}, nil
	case "remove_mark":
		m, err := mark()
		if err != nil {
			return nil, err
		}
		return &RemoveMarkStep{From: j.From, To: j.To, Mark: m}, nil
	case "set_attrs":
		if len(j.Attrs) == 0 {
			return nil, errorf("invalid_step", "set_attrs needs a position and attributes")
		}
		attrs := map[string]any{}
		for k, v := range j.Attrs {
			attrs[k] = normalize(v)
		}
		return &SetAttrsStep{Pos: j.Pos, Attrs: attrs}, nil
	}
	return nil, errorf("invalid_step", "unknown step type %q", j.StepType)
}

// ReplaceStep replaces From–To with Slice. Structure steps only move
// structure: they fail if they would overwrite content.
type ReplaceStep struct {
	From, To  int
	Slice     Slice
	Structure bool
}

func (s *ReplaceStep) Apply(doc *Node) (*Node, error) {
	if s.Structure && contentBetween(doc, s.From, s.To) {
		return nil, errorf("invalid_step", "structure replace would overwrite content")
	}
	return doc.Replace(s.From, s.To, s.Slice)
}

func (s *ReplaceStep) StepMap() StepMap { return NewStepMap(s.From, s.To-s.From, s.Slice.Size()) }

func (s *ReplaceStep) Invert(doc *Node) (Step, error) {
	slice, err := doc.Slice(s.From, s.To)
	if err != nil {
		return nil, err
	}
	return &ReplaceStep{From: s.From, To: s.From + s.Slice.Size(), Slice: slice}, nil
}

func (s *ReplaceStep) Map(m Mapper) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if from.DeletedAcross() && to.DeletedAcross() {
		return nil
	}
	return &ReplaceStep{From: from.Pos, To: max(from.Pos, to.Pos), Slice: s.Slice, Structure: s.Structure}
}

func (s *ReplaceStep) json() map[string]any {
	return withSlice(map[string]any{"stepType": "replace", "from": s.From, "to": s.To}, s.Slice, s.Structure)
}

// contentBetween reports whether from–to holds more than node boundaries.
func contentBetween(doc *Node, from, to int) bool {
	rf, err := doc.Resolve(from)
	if err != nil {
		return true
	}
	dist := to - from
	depth := rf.Depth
	for dist > 0 && depth > 0 && rf.IndexAfter(depth) == rf.Node(depth).childCount() {
		depth--
		dist--
	}
	if dist > 0 {
		next := rf.Node(depth).maybeChild(rf.IndexAfter(depth))
		for dist > 0 {
			if next == nil || next.IsLeaf() {
				return true
			}
			next = next.Content.first()
			dist--
		}
	}
	return false
}

func (n *Node) maybeChild(i int) *Node {
	if i < len(n.Content.nodes) {
		return n.Content.nodes[i]
	}
	return nil
}

func (f Fragment) first() *Node {
	if len(f.nodes) == 0 {
		return nil
	}
	return f.nodes[0]
}

// ReplaceAroundStep replaces From–To with Slice, keeping GapFrom–GapTo of
// the document and putting it at Insert in the slice: it wraps or unwraps
// content (lists, quotes) without touching it.
type ReplaceAroundStep struct {
	From, To, GapFrom, GapTo int
	Slice                    Slice
	Insert                   int
	Structure                bool
}

func (s *ReplaceAroundStep) Apply(doc *Node) (*Node, error) {
	if s.Structure && (contentBetween(doc, s.From, s.GapFrom) || contentBetween(doc, s.GapTo, s.To)) {
		return nil, errorf("invalid_step", "structure gap-replace would overwrite content")
	}
	gap, err := doc.Slice(s.GapFrom, s.GapTo)
	if err != nil {
		return nil, err
	}
	if gap.OpenStart != 0 || gap.OpenEnd != 0 {
		return nil, errorf("invalid_step", "gap is not a flat range")
	}
	inserted, ok := s.Slice.insertAt(s.Insert, gap.Content)
	if !ok {
		return nil, errorf("invalid_step", "content does not fit in gap")
	}
	return doc.Replace(s.From, s.To, inserted)
}

func (s *ReplaceAroundStep) StepMap() StepMap {
	return NewStepMap(s.From, s.GapFrom-s.From, s.Insert, s.GapTo, s.To-s.GapTo, s.Slice.Size()-s.Insert)
}

func (s *ReplaceAroundStep) Invert(doc *Node) (Step, error) {
	gap := s.GapTo - s.GapFrom
	whole, err := doc.Slice(s.From, s.To)
	if err != nil {
		return nil, err
	}
	removed, err := whole.removeBetween(s.GapFrom-s.From, s.GapTo-s.From)
	if err != nil {
		return nil, err
	}
	return &ReplaceAroundStep{From: s.From, To: s.From + s.Slice.Size() + gap, GapFrom: s.From + s.Insert, GapTo: s.From + s.Insert + gap,
		Slice: removed, Insert: s.GapFrom - s.From, Structure: s.Structure}, nil
}

func (s *ReplaceAroundStep) Map(m Mapper) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	gapFrom := from.Pos
	if s.From != s.GapFrom {
		gapFrom = m.Map(s.GapFrom, -1)
	}
	gapTo := to.Pos
	if s.To != s.GapTo {
		gapTo = m.Map(s.GapTo, 1)
	}
	if (from.DeletedAcross() && to.DeletedAcross()) || gapFrom < from.Pos || gapTo > to.Pos {
		return nil
	}
	return &ReplaceAroundStep{From: from.Pos, To: to.Pos, GapFrom: gapFrom, GapTo: gapTo, Slice: s.Slice, Insert: s.Insert, Structure: s.Structure}
}

func (s *ReplaceAroundStep) json() map[string]any {
	return withSlice(map[string]any{"stepType": "replace_around", "from": s.From, "to": s.To, "gapFrom": s.GapFrom, "gapTo": s.GapTo,
		"insert": s.Insert}, s.Slice, s.Structure)
}

// AddMarkStep adds a mark to the inline content of From–To.
type AddMarkStep struct {
	From, To int
	Mark     Mark
}

func (s *AddMarkStep) Apply(doc *Node) (*Node, error) {
	return applyMark(doc, s.From, s.To, func(n *Node) *Node { return n.mark(addMark(n.Marks, s.Mark)) }, s.Mark)
}

func (s *AddMarkStep) StepMap() StepMap { return EmptyMap }

func (s *AddMarkStep) Invert(*Node) (Step, error) {
	return &RemoveMarkStep{From: s.From, To: s.To, Mark: s.Mark}, nil
}

func (s *AddMarkStep) Map(m Mapper) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if (from.Deleted() && to.Deleted()) || from.Pos >= to.Pos {
		return nil
	}
	return &AddMarkStep{From: from.Pos, To: to.Pos, Mark: s.Mark}
}

func (s *AddMarkStep) json() map[string]any {
	return map[string]any{"stepType": "add_mark", "from": s.From, "to": s.To, "mark": markJSON{Type: s.Mark.Type.Name, Attrs: s.Mark.Attrs}}
}

// RemoveMarkStep removes a mark from the inline content of From–To.
type RemoveMarkStep struct {
	From, To int
	Mark     Mark
}

func (s *RemoveMarkStep) Apply(doc *Node) (*Node, error) {
	return applyMark(doc, s.From, s.To, func(n *Node) *Node { return n.mark(removeMark(n.Marks, s.Mark)) }, Mark{})
}

func (s *RemoveMarkStep) StepMap() StepMap { return EmptyMap }

func (s *RemoveMarkStep) Invert(*Node) (Step, error) {
	return &AddMarkStep{From: s.From, To: s.To, Mark: s.Mark}, nil
}

func (s *RemoveMarkStep) Map(m Mapper) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if (from.Deleted() && to.Deleted()) || from.Pos >= to.Pos {
		return nil
	}
	return &RemoveMarkStep{From: from.Pos, To: to.Pos, Mark: s.Mark}
}

func (s *RemoveMarkStep) json() map[string]any {
	return map[string]any{"stepType": "remove_mark", "from": s.From, "to": s.To, "mark": markJSON{Type: s.Mark.Type.Name, Attrs: s.Mark.Attrs}}
}

// applyMark changes the marks of the inline nodes of from–to whose parent
// allows the mark (adding; Type nil when removing).
func applyMark(doc *Node, from, to int, f func(*Node) *Node, adding Mark) (*Node, error) {
	old, err := doc.Slice(from, to)
	if err != nil {
		return nil, err
	}
	rf, err := doc.Resolve(from)
	if err != nil {
		return nil, err
	}
	parent := rf.Node(rf.SharedDepth(to))
	content := mapFragment(old.Content, func(n, parent *Node) *Node {
		if !n.IsLeaf() || (adding.Type != nil && !parent.Type.AllowsMark(adding.Type)) {
			return n
		}
		return f(n)
	}, parent)
	return doc.Replace(from, to, Slice{Content: content, OpenStart: old.OpenStart, OpenEnd: old.OpenEnd})
}

func mapFragment(f Fragment, fn func(n, parent *Node) *Node, parent *Node) Fragment {
	out := make([]*Node, len(f.nodes))
	for i, c := range f.nodes {
		if c.Content.size > 0 {
			c = c.copy(mapFragment(c.Content, fn, c))
		}
		if c.IsInline() {
			c = fn(c, parent)
		}
		out[i] = c
	}
	return NewFragment(out)
}

// SetAttrsStep changes attributes of the node at Pos (heading level, task
// checked…), leaving the others.
type SetAttrsStep struct {
	Pos   int
	Attrs map[string]any
}

func (s *SetAttrsStep) Apply(doc *Node) (*Node, error) {
	node := doc.NodeAt(s.Pos)
	if node == nil || node.IsText() {
		return nil, errorf("invalid_step", "no node at %d", s.Pos)
	}
	attrs := maps.Clone(node.Attrs)
	if attrs == nil {
		attrs = map[string]any{}
	}
	maps.Copy(attrs, s.Attrs)
	checked, err := computeAttrs(node.Type.Attrs, attrs, node.Type.schema.MaxText, "node "+node.Type.Name)
	if err != nil {
		return nil, err
	}
	// The new node is open at its end: the old one's content joins it.
	updated := &Node{Type: node.Type, Attrs: checked, Marks: node.Marks}
	open := 1
	if node.IsLeaf() {
		open = 0
	}
	return doc.Replace(s.Pos, s.Pos+1, Slice{Content: NewFragment([]*Node{updated}), OpenStart: 0, OpenEnd: open})
}

func (s *SetAttrsStep) StepMap() StepMap { return EmptyMap }

func (s *SetAttrsStep) Invert(doc *Node) (Step, error) {
	node := doc.NodeAt(s.Pos)
	if node == nil {
		return nil, errorf("invalid_step", "no node at %d", s.Pos)
	}
	old := map[string]any{}
	for k := range s.Attrs {
		old[k] = node.Attrs[k]
	}
	return &SetAttrsStep{Pos: s.Pos, Attrs: old}, nil
}

func (s *SetAttrsStep) Map(m Mapper) Step {
	r := m.MapResult(s.Pos, 1)
	if r.DeletedAfter() {
		return nil
	}
	return &SetAttrsStep{Pos: r.Pos, Attrs: s.Attrs}
}

func (s *SetAttrsStep) json() map[string]any {
	return map[string]any{"stepType": "set_attrs", "pos": s.Pos, "attrs": s.Attrs}
}

// StepsEqual reports whether two steps are the same.
func StepsEqual(a, b Step) bool {
	ja, _ := MarshalStep(a)
	jb, _ := MarshalStep(b)
	var x, y any
	_ = json.Unmarshal(ja, &x)
	_ = json.Unmarshal(jb, &y)
	return reflect.DeepEqual(x, y)
}
