package document

import (
	"encoding/json"
	"reflect"
	"slices"
	"unicode/utf16"
)

// Node is an immutable document node. Text nodes hold Text (positions
// count UTF-16 code units, as in JavaScript); others hold Content.
type Node struct {
	Type    *NodeType
	Attrs   map[string]any
	Content Fragment
	Text    string
	Marks   []Mark
	tsize   int // UTF-16 length of Text
}

// Mark is a mark on an inline node.
type Mark struct {
	Type  *MarkType
	Attrs map[string]any
}

// Eq reports whether two marks are the same.
func (m Mark) Eq(o Mark) bool { return m.Type == o.Type && reflect.DeepEqual(m.Attrs, o.Attrs) }

// markSet helpers: sets are sorted by rank and hold one mark per type.
func addMark(set []Mark, m Mark) []Mark {
	out := make([]Mark, 0, len(set)+1)
	placed := false
	for _, x := range set {
		if x.Type == m.Type {
			if x.Eq(m) {
				return set
			}
			continue // replaced by m
		}
		if !placed && m.Type.Rank < x.Type.Rank {
			out = append(out, m)
			placed = true
		}
		out = append(out, x)
	}
	if !placed {
		out = append(out, m)
	}
	return out
}

func removeMark(set []Mark, m Mark) []Mark {
	out := make([]Mark, 0, len(set))
	for _, x := range set {
		if !x.Eq(m) {
			out = append(out, x)
		}
	}
	return out
}

func sameMarks(a, b []Mark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Eq(b[i]) {
			return false
		}
	}
	return true
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// splitsPair reports whether a UTF-16 offset falls inside a surrogate
// pair: no position may.
func splitsPair(s string, offset int) bool {
	n := 0
	for _, r := range s {
		if n >= offset {
			return false
		}
		if r >= 0x10000 {
			n += 2
			if n > offset {
				return true
			}
		} else {
			n++
		}
	}
	return false
}

// cutText returns s[from:to] counted in UTF-16 code units.
func cutText(s string, from, to int) string {
	u := utf16.Encode([]rune(s))
	return string(utf16.Decode(u[from:to]))
}

// NewText makes a text node.
func NewText(s *Schema, text string, marks []Mark) *Node {
	return &Node{Type: s.Nodes["text"], Text: text, Marks: marks, tsize: utf16Len(text)}
}

// IsText reports whether n is a text node.
func (n *Node) IsText() bool { return n.Type.IsText() }

// IsInline reports whether n is inline.
func (n *Node) IsInline() bool { return n.Type.IsInline() }

// IsLeaf reports whether n has no content.
func (n *Node) IsLeaf() bool { return n.Type.IsLeaf() }

// NodeSize is the size of n in positions: a text node's length, 1 for a
// leaf, and its content plus its two boundary tokens otherwise.
func (n *Node) NodeSize() int {
	switch {
	case n.IsText():
		return n.tsize
	case n.IsLeaf():
		return 1
	}
	return n.Content.size + 2
}

func (n *Node) childCount() int   { return len(n.Content.nodes) }
func (n *Node) child(i int) *Node { return n.Content.nodes[i] }

// copy returns n with other content.
func (n *Node) copy(content Fragment) *Node {
	c := *n
	c.Content = content
	return &c
}

func (n *Node) withText(text string) *Node {
	c := *n
	c.Text, c.tsize = text, utf16Len(text)
	return &c
}

func (n *Node) mark(marks []Mark) *Node {
	c := *n
	c.Marks = marks
	return &c
}

// sameMarkup reports whether two nodes have the same type, attributes and
// marks.
func (n *Node) sameMarkup(o *Node) bool {
	return n.Type == o.Type && reflect.DeepEqual(n.Attrs, o.Attrs) && sameMarks(n.Marks, o.Marks)
}

// Eq reports whether two nodes are the same, deeply.
func (n *Node) Eq(o *Node) bool {
	if n == o {
		return true
	}
	return n.sameMarkup(o) && n.Text == o.Text && n.Content.Eq(o.Content)
}

// cut returns the part of n between from and to (relative to its content,
// or its text).
func (n *Node) cut(from, to int) *Node {
	if n.IsText() {
		if from == 0 && to == n.tsize {
			return n
		}
		return n.withText(cutText(n.Text, from, to))
	}
	if from == 0 && to == n.Content.size {
		return n
	}
	return n.copy(n.Content.cut(from, to))
}

// NodeAt returns the node starting at pos, or nil.
func (n *Node) NodeAt(pos int) *Node {
	node := n
	for {
		index, offset := node.Content.findIndex(pos)
		if index >= len(node.Content.nodes) {
			return nil
		}
		child := node.Content.nodes[index]
		if offset == pos || child.IsText() {
			return child
		}
		pos -= offset + 1
		node = child
	}
}

// Slice cuts the document between two positions into an open slice.
func (n *Node) Slice(from, to int) (Slice, error) {
	if from == to {
		return Slice{}, nil
	}
	rf, err := n.Resolve(from)
	if err != nil {
		return Slice{}, err
	}
	rt, err := n.Resolve(to)
	if err != nil {
		return Slice{}, err
	}
	depth := rf.SharedDepth(to)
	start := rf.Start(depth)
	node := rf.Node(depth)
	content := node.Content.cut(rf.Pos-start, rt.Pos-start)
	return Slice{Content: content, OpenStart: rf.Depth - depth, OpenEnd: rt.Depth - depth}, nil
}

// Fragment is a node's content: a sequence of nodes and its size.
type Fragment struct {
	nodes []*Node
	size  int
}

// NewFragment makes a fragment, joining adjacent text nodes with the same
// marks.
func NewFragment(nodes []*Node) Fragment {
	var out []*Node
	size := 0
	for _, c := range nodes {
		size += c.NodeSize()
		if c.IsText() && len(out) > 0 && out[len(out)-1].IsText() && out[len(out)-1].sameMarkup(c) {
			out[len(out)-1] = out[len(out)-1].withText(out[len(out)-1].Text + c.Text)
			continue
		}
		out = append(out, c)
	}
	return Fragment{nodes: out, size: size}
}

// Nodes returns the fragment's children.
func (f Fragment) Nodes() []*Node { return f.nodes }

// Size is the fragment's size in positions.
func (f Fragment) Size() int { return f.size }

// Eq reports whether two fragments are the same.
func (f Fragment) Eq(o Fragment) bool {
	if len(f.nodes) != len(o.nodes) {
		return false
	}
	for i := range f.nodes {
		if !f.nodes[i].Eq(o.nodes[i]) {
			return false
		}
	}
	return true
}

func (f Fragment) append(o Fragment) Fragment {
	if o.size == 0 {
		return f
	}
	if f.size == 0 {
		return o
	}
	return NewFragment(append(slices.Clone(f.nodes), o.nodes...))
}

func (f Fragment) cut(from, to int) Fragment {
	if from == 0 && to == f.size {
		return f
	}
	var out []*Node
	if to > from {
		pos := 0
		for _, child := range f.nodes {
			end := pos + child.NodeSize()
			if end > from {
				if pos < from || end > to {
					if child.IsText() {
						child = child.cut(max(0, from-pos), min(child.tsize, to-pos))
					} else {
						child = child.cut(max(0, from-pos-1), min(child.Content.size, to-pos-1))
					}
				}
				out = append(out, child)
			}
			pos = end
			if pos >= to {
				break
			}
		}
	}
	return NewFragment(out)
}

func (f Fragment) replaceChild(i int, n *Node) Fragment {
	nodes := slices.Clone(f.nodes)
	nodes[i] = n
	return NewFragment(nodes)
}

// findIndex finds the child containing pos: its index and start offset.
// At the end, the index is the child count.
func (f Fragment) findIndex(pos int) (int, int) {
	if pos == 0 {
		return 0, 0
	}
	if pos == f.size {
		return len(f.nodes), pos
	}
	cur := 0
	for i, c := range f.nodes {
		end := cur + c.NodeSize()
		if end >= pos {
			if end == pos {
				return i + 1, end
			}
			return i, cur
		}
		cur = end
	}
	return len(f.nodes), f.size
}

// Slice is a piece of a document: content whose first and last OpenStart
// and OpenEnd levels are open (cut through).
type Slice struct {
	Content   Fragment
	OpenStart int
	OpenEnd   int
}

// Size is the positions the slice adds when inserted.
func (s Slice) Size() int { return s.Content.size - s.OpenStart - s.OpenEnd }

// Eq reports whether two slices are the same.
func (s Slice) Eq(o Slice) bool {
	return s.OpenStart == o.OpenStart && s.OpenEnd == o.OpenEnd && s.Content.Eq(o.Content)
}

func (s Slice) insertAt(pos int, f Fragment) (Slice, bool) {
	content, ok := insertInto(s.Content, pos+s.OpenStart, f)
	if !ok {
		return Slice{}, false
	}
	return Slice{Content: content, OpenStart: s.OpenStart, OpenEnd: s.OpenEnd}, true
}

// insertInto puts insert at dist in content, however deep; the replace
// that uses the result checks it.
func insertInto(content Fragment, dist int, insert Fragment) (Fragment, bool) {
	index, offset := content.findIndex(dist)
	var child *Node
	if index < len(content.nodes) {
		child = content.nodes[index]
	}
	if offset == dist || (child != nil && child.IsText()) {
		return content.cut(0, dist).append(insert).append(content.cut(dist, content.size)), true
	}
	if child == nil {
		return Fragment{}, false
	}
	inner, ok := insertInto(child.Content, dist-offset-1, insert)
	if !ok {
		return Fragment{}, false
	}
	return content.replaceChild(index, child.copy(inner)), true
}

func (s Slice) removeBetween(from, to int) (Slice, error) {
	content, err := removeRange(s.Content, from+s.OpenStart, to+s.OpenStart)
	if err != nil {
		return Slice{}, err
	}
	return Slice{Content: content, OpenStart: s.OpenStart, OpenEnd: s.OpenEnd}, nil
}

func removeRange(content Fragment, from, to int) (Fragment, error) {
	index, offset := content.findIndex(from)
	indexTo, offsetTo := content.findIndex(to)
	var child *Node
	if index < len(content.nodes) {
		child = content.nodes[index]
	}
	if offset == from || (child != nil && child.IsText()) {
		if offsetTo != to && !content.nodes[indexTo].IsText() {
			return Fragment{}, errorf("invalid_step", "removing a non-flat range")
		}
		return content.cut(0, from).append(content.cut(to, content.size)), nil
	}
	if index != indexTo {
		return Fragment{}, errorf("invalid_step", "removing a non-flat range")
	}
	inner, err := removeRange(child.Content, from-offset-1, to-offset-1)
	if err != nil {
		return Fragment{}, err
	}
	return content.replaceChild(index, child.copy(inner)), nil
}

// ResolvedPos is a position with its path through the document.
type ResolvedPos struct {
	Pos          int
	Depth        int
	ParentOffset int
	path         []pathStep
}

type pathStep struct {
	node   *Node
	index  int
	offset int // start of the child at index, in the node's content
}

// Resolve resolves pos in the document n.
func (n *Node) Resolve(pos int) (*ResolvedPos, error) {
	if pos < 0 || pos > n.Content.size {
		return nil, errorf("invalid_step", "position %d out of range", pos)
	}
	var path []pathStep
	start := 0
	parentOffset := pos
	for node := n; ; {
		index, offset := node.Content.findIndex(parentOffset)
		rem := parentOffset - offset
		path = append(path, pathStep{node: node, index: index, offset: start + offset})
		if rem == 0 {
			break
		}
		child := node.Content.nodes[index]
		if child.IsText() {
			if splitsPair(child.Text, rem) {
				return nil, errorf("invalid_step", "position %d splits a character", pos)
			}
			break
		}
		node = child
		parentOffset = rem - 1
		start += offset + 1
	}
	return &ResolvedPos{Pos: pos, Depth: len(path) - 1, ParentOffset: parentOffset, path: path}, nil
}

func (r *ResolvedPos) resolveDepth(d int) int {
	if d < 0 {
		return r.Depth + d
	}
	return d
}

// Node is the ancestor node at depth d.
func (r *ResolvedPos) Node(d int) *Node { return r.path[r.resolveDepth(d)].node }

// Parent is the node the position is directly in.
func (r *ResolvedPos) Parent() *Node { return r.Node(r.Depth) }

// Index is the index of the position in its ancestor at depth d.
func (r *ResolvedPos) Index(d int) int { return r.path[r.resolveDepth(d)].index }

// IndexAfter is Index, plus one when the position points into a node.
func (r *ResolvedPos) IndexAfter(d int) int {
	d = r.resolveDepth(d)
	if d == r.Depth && r.TextOffset() == 0 {
		return r.Index(d)
	}
	return r.Index(d) + 1
}

// Start is the start of the content of the ancestor at depth d.
func (r *ResolvedPos) Start(d int) int {
	d = r.resolveDepth(d)
	if d == 0 {
		return 0
	}
	return r.path[d-1].offset + 1
}

// End is the end of the content of the ancestor at depth d.
func (r *ResolvedPos) End(d int) int {
	d = r.resolveDepth(d)
	return r.Start(d) + r.Node(d).Content.size
}

// Before is the position before the ancestor at depth d (d ≥ 1).
func (r *ResolvedPos) Before(d int) int {
	d = r.resolveDepth(d)
	if d == r.Depth+1 {
		return r.Pos
	}
	return r.path[d-1].offset
}

// After is the position after the ancestor at depth d (d ≥ 1).
func (r *ResolvedPos) After(d int) int {
	d = r.resolveDepth(d)
	if d == r.Depth+1 {
		return r.Pos
	}
	return r.path[d-1].offset + r.path[d].node.NodeSize()
}

// TextOffset is the offset into the text node the position points into.
func (r *ResolvedPos) TextOffset() int { return r.Pos - r.path[r.Depth].offset }

// NodeAfter is the node right after the position, cut at it for text.
func (r *ResolvedPos) NodeAfter() *Node {
	parent, index := r.Parent(), r.Index(r.Depth)
	if index == parent.childCount() {
		return nil
	}
	dOff, child := r.TextOffset(), parent.child(index)
	if dOff > 0 {
		return child.cut(dOff, child.tsize)
	}
	return child
}

// NodeBefore is the node right before the position, cut at it for text.
func (r *ResolvedPos) NodeBefore() *Node {
	index := r.Index(r.Depth)
	dOff := r.TextOffset()
	if dOff > 0 {
		return r.Parent().child(index).cut(0, dOff)
	}
	if index == 0 {
		return nil
	}
	return r.Parent().child(index - 1)
}

// SharedDepth is the depth of the deepest ancestor containing pos too.
func (r *ResolvedPos) SharedDepth(pos int) int {
	for d := r.Depth; d > 0; d-- {
		if r.Start(d) <= pos && r.End(d) >= pos {
			return d
		}
	}
	return 0
}

// --- JSON -----------------------------------------------------------------------------

// nodeJSON is a node's JSON form.
type nodeJSON struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []*nodeJSON    `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []markJSON     `json:"marks,omitempty"`
}

type markJSON struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// MarshalJSON writes the node in the documented format.
func (n *Node) MarshalJSON() ([]byte, error) { return json.Marshal(n.toJSON()) }

func (n *Node) toJSON() *nodeJSON {
	j := &nodeJSON{Type: n.Type.Name, Attrs: n.Attrs, Text: n.Text}
	for _, c := range n.Content.nodes {
		j.Content = append(j.Content, c.toJSON())
	}
	for _, m := range n.Marks {
		j.Marks = append(j.Marks, markJSON{Type: m.Type.Name, Attrs: m.Attrs})
	}
	return j
}

// ParseJSON reads a document (or any node) and checks it against the
// schema: known types, valid attributes and marks, valid content, no empty
// text nodes, and the size and depth limits.
func (s *Schema) ParseJSON(raw []byte) (*Node, error) {
	if s.MaxBytes > 0 && len(raw) > s.MaxBytes {
		return nil, errorf("too_large", "document over %d bytes", s.MaxBytes)
	}
	var j nodeJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, errorf("invalid_document", "%v", err)
	}
	return s.fromJSON(&j, 0)
}

func (s *Schema) fromJSON(j *nodeJSON, depth int) (*Node, error) {
	if s.MaxDepth > 0 && depth > s.MaxDepth {
		return nil, errorf("too_large", "document nested deeper than %d", s.MaxDepth)
	}
	t := s.Nodes[j.Type]
	if t == nil {
		return nil, errorf("invalid_document", "unknown node type %q", j.Type)
	}
	marks, err := s.marksFromJSON(j.Marks)
	if err != nil {
		return nil, err
	}
	if t.IsText() {
		if j.Text == "" || len(j.Content) > 0 || len(j.Attrs) > 0 {
			return nil, errorf("invalid_document", "a text node holds non-empty text only")
		}
		return NewText(s, j.Text, marks), nil
	}
	if j.Text != "" {
		return nil, errorf("invalid_document", "node %s holds no text", t.Name)
	}
	if len(marks) > 0 && !t.IsInline() {
		return nil, errorf("invalid_document", "node %s takes no marks", t.Name)
	}
	attrs, err := computeAttrs(t.Attrs, j.Attrs, s.MaxText, "node "+t.Name)
	if err != nil {
		return nil, err
	}
	children := make([]*Node, 0, len(j.Content))
	for _, c := range j.Content {
		child, err := s.fromJSON(c, depth+1)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	content := NewFragment(children)
	if err := t.checkContent(content); err != nil {
		return nil, err
	}
	return &Node{Type: t, Attrs: attrs, Content: content, Marks: marks}, nil
}

func (s *Schema) marksFromJSON(js []markJSON) ([]Mark, error) {
	var out []Mark
	for _, mj := range js {
		mt := s.Marks[mj.Type]
		if mt == nil {
			return nil, errorf("invalid_document", "unknown mark %q", mj.Type)
		}
		attrs, err := computeAttrs(mt.Attrs, mj.Attrs, s.MaxText, "mark "+mt.Name)
		if err != nil {
			return nil, err
		}
		out = addMark(out, Mark{Type: mt, Attrs: attrs})
	}
	return out, nil
}

// sliceJSON is a slice's JSON form.
type sliceJSON struct {
	Content   []*nodeJSON `json:"content,omitempty"`
	OpenStart int         `json:"openStart,omitempty"`
	OpenEnd   int         `json:"openEnd,omitempty"`
}

func (s Slice) toJSON() sliceJSON {
	j := sliceJSON{OpenStart: s.OpenStart, OpenEnd: s.OpenEnd}
	for _, c := range s.Content.nodes {
		j.Content = append(j.Content, c.toJSON())
	}
	return j
}

func (s *Schema) sliceFromJSON(j sliceJSON) (Slice, error) {
	var nodes []*Node
	for _, c := range j.Content {
		// Nodes of a slice may be open: their content isn't checked here,
		// the replace that inserts them checks the result.
		n, err := s.looseFromJSON(c, 1)
		if err != nil {
			return Slice{}, err
		}
		nodes = append(nodes, n)
	}
	content := NewFragment(nodes)
	// A slice is open at most as deep as its first and last descendants go.
	if j.OpenStart < 0 || j.OpenEnd < 0 || j.OpenStart > edgeDepth(content, true) || j.OpenEnd > edgeDepth(content, false) {
		return Slice{}, errorf("invalid_step", "slice open deeper than its content")
	}
	return Slice{Content: content, OpenStart: j.OpenStart, OpenEnd: j.OpenEnd}, nil
}

// edgeDepth is how deep the first (or last) non-leaf descendants go.
func edgeDepth(f Fragment, first bool) int {
	d := 0
	for len(f.nodes) > 0 {
		n := f.nodes[len(f.nodes)-1]
		if first {
			n = f.nodes[0]
		}
		if n.IsLeaf() {
			break
		}
		d++
		f = n.Content
	}
	return d
}

// looseFromJSON is fromJSON without the content check, for open slices.
func (s *Schema) looseFromJSON(j *nodeJSON, depth int) (*Node, error) {
	if s.MaxDepth > 0 && depth > s.MaxDepth {
		return nil, errorf("too_large", "slice nested deeper than %d", s.MaxDepth)
	}
	t := s.Nodes[j.Type]
	if t == nil {
		return nil, errorf("invalid_document", "unknown node type %q", j.Type)
	}
	marks, err := s.marksFromJSON(j.Marks)
	if err != nil {
		return nil, err
	}
	if t.IsText() {
		if j.Text == "" {
			return nil, errorf("invalid_document", "empty text node")
		}
		return NewText(s, j.Text, marks), nil
	}
	attrs, err := computeAttrs(t.Attrs, j.Attrs, s.MaxText, "node "+t.Name)
	if err != nil {
		return nil, err
	}
	children := make([]*Node, 0, len(j.Content))
	for _, c := range j.Content {
		child, err := s.looseFromJSON(c, depth+1)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return &Node{Type: t, Attrs: attrs, Content: NewFragment(children), Marks: marks}, nil
}

// Check verifies the whole tree against the schema: what steps produce
// must be checked, since a step's slice may carry nodes with invalid
// content (the replace only checks the nodes it joins).
func (n *Node) Check() error {
	return n.check(0)
}

func (n *Node) check(depth int) error {
	s := n.Type.schema
	if s.MaxDepth > 0 && depth > s.MaxDepth {
		return errorf("too_large", "document nested deeper than %d", s.MaxDepth)
	}
	if n.IsText() {
		if n.Text == "" {
			return errorf("invalid_document", "empty text node")
		}
		return nil
	}
	if err := n.Type.checkContent(n.Content); err != nil {
		return err
	}
	for _, c := range n.Content.nodes {
		if err := c.check(depth + 1); err != nil {
			return err
		}
	}
	return nil
}
