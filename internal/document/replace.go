package document

// The replace algorithm: put a slice between two positions, joining the
// open sides of the slice with the content around them (ProseMirror's
// model/replace.js).

// Replace replaces the range from–to of the document with slice.
func (n *Node) Replace(from, to int, slice Slice) (*Node, error) {
	rf, err := n.Resolve(from)
	if err != nil {
		return nil, err
	}
	rt, err := n.Resolve(to)
	if err != nil {
		return nil, err
	}
	if slice.OpenStart > rf.Depth {
		return nil, errorf("invalid_step", "inserted content deeper than the insertion position")
	}
	if rf.Depth-slice.OpenStart != rt.Depth-slice.OpenEnd {
		return nil, errorf("invalid_step", "inconsistent open depths")
	}
	return replaceOuter(rf, rt, slice, 0)
}

func replaceOuter(rf, rt *ResolvedPos, slice Slice, depth int) (*Node, error) {
	index, node := rf.Index(depth), rf.Node(depth)
	switch {
	case index == rt.Index(depth) && depth < rf.Depth-slice.OpenStart:
		inner, err := replaceOuter(rf, rt, slice, depth+1)
		if err != nil {
			return nil, err
		}
		return node.copy(node.Content.replaceChild(index, inner)), nil
	case slice.Content.size == 0:
		content, err := replaceTwoWay(rf, rt, depth)
		if err != nil {
			return nil, err
		}
		return closeNode(node, content)
	case slice.OpenStart == 0 && slice.OpenEnd == 0 && rf.Depth == depth && rt.Depth == depth:
		parent := rf.Parent()
		content := parent.Content
		return closeNode(parent, content.cut(0, rf.ParentOffset).append(slice.Content).append(content.cut(rt.ParentOffset, content.size)))
	default:
		start, end, err := prepareSliceForReplace(slice, rf)
		if err != nil {
			return nil, err
		}
		content, err := replaceThreeWay(rf, start, end, rt, depth)
		if err != nil {
			return nil, err
		}
		return closeNode(node, content)
	}
}

func checkJoin(main, sub *Node) error {
	if !sub.Type.compatibleContent(main.Type) {
		return errorf("invalid_step", "cannot join %s onto %s", sub.Type.Name, main.Type.Name)
	}
	return nil
}

func joinable(before, after *ResolvedPos, depth int) (*Node, error) {
	node := before.Node(depth)
	if err := checkJoin(node, after.Node(depth)); err != nil {
		return nil, err
	}
	return node, nil
}

func addNode(child *Node, target []*Node) []*Node {
	last := len(target) - 1
	if last >= 0 && child.IsText() && child.sameMarkup(target[last]) {
		target[last] = child.withText(target[last].Text + child.Text)
		return target
	}
	return append(target, child)
}

func addRange(start, end *ResolvedPos, depth int, target []*Node) []*Node {
	var node *Node
	if end != nil {
		node = end.Node(depth)
	} else {
		node = start.Node(depth)
	}
	startIndex, endIndex := 0, node.childCount()
	if end != nil {
		endIndex = end.Index(depth)
	}
	if start != nil {
		startIndex = start.Index(depth)
		if start.Depth > depth {
			startIndex++
		} else if start.TextOffset() > 0 {
			target = addNode(start.NodeAfter(), target)
			startIndex++
		}
	}
	for i := startIndex; i < endIndex; i++ {
		target = addNode(node.child(i), target)
	}
	if end != nil && end.Depth == depth && end.TextOffset() > 0 {
		target = addNode(end.NodeBefore(), target)
	}
	return target
}

func closeNode(node *Node, content Fragment) (*Node, error) {
	if err := node.Type.checkContent(content); err != nil {
		return nil, err
	}
	return node.copy(content), nil
}

func replaceThreeWay(rf, start, end, rt *ResolvedPos, depth int) (Fragment, error) {
	var openStart, openEnd *Node
	var err error
	if rf.Depth > depth {
		if openStart, err = joinable(rf, start, depth+1); err != nil {
			return Fragment{}, err
		}
	}
	if rt.Depth > depth {
		if openEnd, err = joinable(end, rt, depth+1); err != nil {
			return Fragment{}, err
		}
	}
	content := addRange(nil, rf, depth, nil)
	if openStart != nil && openEnd != nil && start.Index(depth) == end.Index(depth) {
		if err := checkJoin(openStart, openEnd); err != nil {
			return Fragment{}, err
		}
		inner, err := replaceThreeWay(rf, start, end, rt, depth+1)
		if err != nil {
			return Fragment{}, err
		}
		closed, err := closeNode(openStart, inner)
		if err != nil {
			return Fragment{}, err
		}
		content = addNode(closed, content)
	} else {
		if openStart != nil {
			inner, err := replaceTwoWay(rf, start, depth+1)
			if err != nil {
				return Fragment{}, err
			}
			closed, err := closeNode(openStart, inner)
			if err != nil {
				return Fragment{}, err
			}
			content = addNode(closed, content)
		}
		content = addRange(start, end, depth, content)
		if openEnd != nil {
			inner, err := replaceTwoWay(end, rt, depth+1)
			if err != nil {
				return Fragment{}, err
			}
			closed, err := closeNode(openEnd, inner)
			if err != nil {
				return Fragment{}, err
			}
			content = addNode(closed, content)
		}
	}
	content = addRange(rt, nil, depth, content)
	return NewFragment(content), nil
}

func replaceTwoWay(rf, rt *ResolvedPos, depth int) (Fragment, error) {
	content := addRange(nil, rf, depth, nil)
	if rf.Depth > depth {
		t, err := joinable(rf, rt, depth+1)
		if err != nil {
			return Fragment{}, err
		}
		inner, err := replaceTwoWay(rf, rt, depth+1)
		if err != nil {
			return Fragment{}, err
		}
		closed, err := closeNode(t, inner)
		if err != nil {
			return Fragment{}, err
		}
		content = addNode(closed, content)
	}
	content = addRange(rt, nil, depth, content)
	return NewFragment(content), nil
}

func prepareSliceForReplace(slice Slice, along *ResolvedPos) (*ResolvedPos, *ResolvedPos, error) {
	extra := along.Depth - slice.OpenStart
	parent := along.Node(extra)
	node := parent.copy(slice.Content)
	for i := extra - 1; i >= 0; i-- {
		node = along.Node(i).copy(NewFragment([]*Node{node}))
	}
	start, err := node.Resolve(slice.OpenStart + extra)
	if err != nil {
		return nil, nil, err
	}
	end, err := node.Resolve(node.Content.size - slice.OpenEnd - extra)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}
