package document

// Position mapping (ProseMirror's transform/map.js): a step's map tells
// where positions of the document before it end up after it, so steps and
// selections made against one version can be carried to a later one.

// Flags of a mapped position.
const (
	delBefore = 1 << iota // the content before the position was deleted
	delAfter              // the content after it was
	delAcross             // the position was inside deleted content
	delSide               // the side the position associated with was deleted
)

const recoverFactor = 1 << 16

func makeRecover(index, offset int) int { return index + offset*recoverFactor }
func recoverIndex(v int) int            { return v & 0xffff }
func recoverOffset(v int) int           { return (v - (v & 0xffff)) / recoverFactor }

// MapResult is a mapped position and what happened around it.
type MapResult struct {
	Pos     int
	delInfo int
	recover int // -1: none
}

// Deleted reports whether the position's side was deleted.
func (r MapResult) Deleted() bool { return r.delInfo&delSide != 0 }

// DeletedBefore reports whether the content before the position was deleted.
func (r MapResult) DeletedBefore() bool { return r.delInfo&(delBefore|delAcross) != 0 }

// DeletedAfter reports whether the content after the position was deleted.
func (r MapResult) DeletedAfter() bool { return r.delInfo&(delAfter|delAcross) != 0 }

// DeletedAcross reports whether the position was inside deleted content.
func (r MapResult) DeletedAcross() bool { return r.delInfo&delAcross != 0 }

// StepMap maps positions through one step: ranges of (start, old size, new
// size) triples, in order.
type StepMap struct {
	ranges   []int
	inverted bool
}

// EmptyMap maps every position to itself.
var EmptyMap = StepMap{}

// NewStepMap makes a map from (start, old size, new size) triples.
func NewStepMap(ranges ...int) StepMap { return StepMap{ranges: ranges} }

// Invert maps the other way.
func (m StepMap) Invert() StepMap { return StepMap{ranges: m.ranges, inverted: !m.inverted} }

func (m StepMap) recover(value int) int {
	diff, index := 0, recoverIndex(value)
	if !m.inverted {
		for i := 0; i < index; i++ {
			diff += m.ranges[i*3+2] - m.ranges[i*3+1]
		}
	}
	return m.ranges[index*3] + diff + recoverOffset(value)
}

// MapResult maps pos; assoc (-1 or 1) says which side it sticks to when
// content is inserted at it.
func (m StepMap) MapResult(pos, assoc int) MapResult {
	diff := 0
	oldIndex, newIndex := 1, 2
	if m.inverted {
		oldIndex, newIndex = 2, 1
	}
	for i := 0; i < len(m.ranges); i += 3 {
		start := m.ranges[i]
		if m.inverted {
			start -= diff
		}
		if start > pos {
			break
		}
		oldSize, newSize := m.ranges[i+oldIndex], m.ranges[i+newIndex]
		end := start + oldSize
		if pos <= end {
			side := assoc
			if oldSize != 0 {
				switch pos {
				case start:
					side = -1
				case end:
					side = 1
				}
			}
			result := start + diff
			if side >= 0 {
				result += newSize
			}
			rec := -1
			edge := end
			if assoc < 0 {
				edge = start
			}
			if pos != edge {
				rec = makeRecover(i/3, pos-start)
			}
			del := delAcross
			switch pos {
			case start:
				del = delAfter
			case end:
				del = delBefore
			}
			if pos != edge {
				del |= delSide
			}
			return MapResult{Pos: result, delInfo: del, recover: rec}
		}
		diff += newSize - oldSize
	}
	return MapResult{Pos: pos + diff, recover: -1}
}

// Map maps a position.
func (m StepMap) Map(pos, assoc int) int { return m.MapResult(pos, assoc).Pos }

// Mapping is a sequence of maps, some of which may mirror each other (a
// step and its inverse, around a rebase), so positions deleted by one are
// recovered by the other.
type Mapping struct {
	Maps    []StepMap
	mirrors map[int]int
}

// AppendMap adds a map; mirrors, when ≥ 0, is the index of the map it
// mirrors.
func (m *Mapping) AppendMap(sm StepMap, mirrors int) {
	m.Maps = append(m.Maps, sm)
	if mirrors >= 0 {
		m.SetMirror(len(m.Maps)-1, mirrors)
	}
}

// SetMirror records that maps n and m mirror each other.
func (m *Mapping) SetMirror(n, o int) {
	if m.mirrors == nil {
		m.mirrors = map[int]int{}
	}
	m.mirrors[n] = o
	m.mirrors[o] = n
}

func (m *Mapping) mirror(n int) (int, bool) {
	o, ok := m.mirrors[n]
	return o, ok
}

// Slice returns the maps from..to (to < 0: the end), mirrors kept.
func (m *Mapping) Slice(from, to int) *Mapping {
	if to < 0 {
		to = len(m.Maps)
	}
	out := &Mapping{Maps: m.Maps[from:to]}
	for a, b := range m.mirrors {
		if a >= from && a < to && b >= from && b < to {
			out.SetMirror(a-from, b-from)
		}
	}
	return out
}

// MapResult maps pos through every map, recovering positions through
// mirrored pairs.
func (m *Mapping) MapResult(pos, assoc int) MapResult {
	delInfo := 0
	for i := 0; i < len(m.Maps); i++ {
		r := m.Maps[i].MapResult(pos, assoc)
		if r.recover >= 0 {
			if corr, ok := m.mirror(i); ok && corr > i && corr < len(m.Maps) {
				i = corr
				pos = m.Maps[corr].recover(r.recover)
				continue
			}
		}
		delInfo |= r.delInfo
		pos = r.Pos
	}
	return MapResult{Pos: pos, delInfo: delInfo, recover: -1}
}

// Map maps a position.
func (m *Mapping) Map(pos, assoc int) int { return m.MapResult(pos, assoc).Pos }

// Mapper is what steps map through: a StepMap or a Mapping.
type Mapper interface {
	MapResult(pos, assoc int) MapResult
	Map(pos, assoc int) int
}
