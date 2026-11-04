package document

import (
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// FromText makes a document of plain text: one paragraph per line, line
// breaks kept; empty text is one empty paragraph.
func FromText(s *Schema, text string) *Node {
	para := s.Nodes["paragraph"]
	var blocks []*Node
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		var content []*Node
		if line != "" {
			content = append(content, NewText(s, line, nil))
		}
		blocks = append(blocks, &Node{Type: para, Content: NewFragment(content)})
	}
	// Trailing empty lines carry nothing.
	for len(blocks) > 1 && blocks[len(blocks)-1].Content.size == 0 {
		blocks = blocks[:len(blocks)-1]
	}
	return &Node{Type: s.Top, Content: NewFragment(blocks)}
}

// PlainText is the document as text, for search, notifications and API
// clients that read text: blocks on their own lines, list items marked,
// inline nodes as their key or name.
func PlainText(n *Node) string {
	var b strings.Builder
	writeText(&b, n, "")
	return strings.TrimRight(b.String(), "\n")
}

func writeText(b *strings.Builder, n *Node, prefix string) {
	switch {
	case n.IsText():
		b.WriteString(n.Text)
		return
	case n.Type.Name == "hard_break":
		b.WriteString("\n" + prefix)
		return
	case n.Type.Name == "issue_ref":
		b.WriteString(attrString(n, "key"))
		return
	case n.Type.Name == "mention":
		b.WriteString("@")
		return
	case n.Type.Name == "emoji":
		b.WriteString(":" + attrString(n, "name") + ":")
		return
	case n.Type.Name == "image":
		if alt := attrString(n, "alt"); alt != "" {
			b.WriteString(prefix + alt + "\n")
		}
		return
	case n.Type.Name == "divider":
		b.WriteString(prefix + "---\n")
		return
	case n.Type.IsTextblock():
		b.WriteString(prefix)
		for _, c := range n.Content.nodes {
			writeText(b, c, prefix)
		}
		b.WriteString("\n")
		return
	}
	if n.Type.Name == "table_row" {
		b.WriteString(prefix)
		for i, c := range n.Content.nodes {
			if i > 0 {
				b.WriteString(" | ")
			}
			writeInline(b, c)
		}
		b.WriteString("\n")
		return
	}
	for i, c := range n.Content.nodes {
		p := prefix
		switch n.Type.Name {
		case "bullet_list":
			p = prefix + "- "
		case "ordered_list":
			p = prefix + strconv.Itoa(int(attrFloat(n, "start"))+i) + ". "
		case "task_list":
			p = prefix + "[ ] "
			if c.Attrs["checked"] == true {
				p = prefix + "[x] "
			}
		case "quote":
			p = prefix + "> "
		}
		writeText(b, c, p)
	}
}

// writeInline writes a table cell's text on one line.
func writeInline(b *strings.Builder, n *Node) {
	var inner strings.Builder
	writeText(&inner, n, "")
	b.WriteString(strings.ReplaceAll(strings.TrimRight(inner.String(), "\n"), "\n", " "))
}

func attrString(n *Node, name string) string {
	s, _ := n.Attrs[name].(string)
	return s
}

func attrFloat(n *Node, name string) float64 {
	f, _ := n.Attrs[name].(float64)
	return f
}

// IsEmpty reports whether the document holds nothing: only empty
// textblocks.
func IsEmpty(n *Node) bool {
	if n.IsText() || n.IsLeaf() {
		return false
	}
	for _, c := range n.Content.nodes {
		if !IsEmpty(c) {
			return false
		}
	}
	return true
}

// Attachments lists the attachment ids of the document's images.
func Attachments(n *Node) []uuid.UUID {
	return collect(n, "image", "attachment")
}

// Mentions lists the people a document mentions, once each, in order.
func Mentions(n *Node) []uuid.UUID {
	return collect(n, "mention", "user")
}

// collect lists the uuid attribute attr of the nodes of type name, once each.
func collect(n *Node, name, attr string) []uuid.UUID {
	var out []uuid.UUID
	var walk func(*Node)
	walk = func(n *Node) {
		if n.Type.Name == name {
			if id, err := uuid.Parse(attrString(n, attr)); err == nil && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
		for _, c := range n.Content.nodes {
			walk(c)
		}
	}
	walk(n)
	return out
}
