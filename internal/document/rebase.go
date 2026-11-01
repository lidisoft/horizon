package document

// Rebase carries local steps over remote ones (docs/realtime.md#protocol,
// step 4). Both lists were made against base; the remote ones won. The
// local steps are undone, the remote ones applied, then each local step is
// mapped through what happened since it was made and applied again, if it
// still can be. Each local step's inverse mirrors its rebased self, so
// positions inside content a local step inserted survive.
//
// It returns the document after it all and the local steps as they now
// apply (some may be gone).
func Rebase(base *Node, local, remote []Step) (*Node, []Step, error) {
	docs := make([]*Node, len(local)+1) // docs[i]: before local[i]
	docs[0] = base
	for i, s := range local {
		next, err := s.Apply(docs[i])
		if err != nil {
			return nil, nil, err
		}
		docs[i+1] = next
	}
	doc := docs[len(local)]
	mapping := &Mapping{}
	for i := len(local) - 1; i >= 0; i-- {
		inv, err := local[i].Invert(docs[i])
		if err != nil {
			return nil, nil, err
		}
		if doc, err = inv.Apply(doc); err != nil {
			return nil, nil, err
		}
		mapping.AppendMap(inv.StepMap(), -1)
	}
	for _, s := range remote {
		var err error
		if doc, err = s.Apply(doc); err != nil {
			return nil, nil, err
		}
		mapping.AppendMap(s.StepMap(), -1)
	}
	var rebased []Step
	mapFrom := len(local)
	for _, s := range local {
		mapped := s.Map(mapping.Slice(mapFrom, -1))
		mapFrom--
		if mapped == nil {
			continue
		}
		next, err := mapped.Apply(doc)
		if err != nil {
			continue // it no longer fits: dropped, as ProseMirror does
		}
		doc = next
		mapping.AppendMap(mapped.StepMap(), mapFrom)
		rebased = append(rebased, mapped)
	}
	return doc, rebased, nil
}
