package step

import "slices"

type mixedConicalDAGStyleItem struct {
	MixedConicalAP242SurfaceStyle
	base, contextID, target, presentationContext int64
}

func proveMixedConicalDAGStyles(f *File, incoming map[int64][]int64, shuo deepSHUOSource) ([]MixedConicalAP242SurfaceStyle, OccurrenceSurfaceColor, []byte, error) {
	var override OccurrenceSurfaceColor
	fail := errMixedConicalAP242DAGAppearance
	bodies, contexts, err := colorBodyIDs(f)
	if err != nil || len(bodies) != 2 || bodies[0] == bodies[1] {
		return nil, override, nil, fail
	}
	faceBody := map[int64]int64{}
	for _, body := range bodies {
		faces, err := bodyFaceIDs(f, body)
		if err != nil {
			return nil, override, nil, err
		}
		for _, id := range faces {
			if faceBody[id] != 0 {
				return nil, override, nil, fail
			}
			faceBody[id] = body
		}
	}
	removed := map[int64]bool{shuo.prefix: true, shuo.terminal: true, shuo.pdsID: true, shuo.sdrID: true}
	items := map[int64]mixedConicalDAGStyleItem{}
	var order []int64
	for _, id := range f.Order {
		rep, err := simple(f, id, "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION", 3)
		if err != nil {
			continue
		}
		_, nameErr := textValue(rep.Args[0])
		context, err := ref(rep.Args[2])
		refs, listErr := listRefs(rep.Args[1], 1)
		if nameErr != nil || err != nil || listErr != nil || len(incoming[id]) != 0 || len(order)+len(refs) > 64 {
			return nil, override, nil, fail
		}
		removed[id] = true
		for _, sid := range refs {
			if _, ok := items[sid]; ok {
				return nil, override, nil, fail
			}
			item, err := readMixedConicalDAGStyleItem(f, incoming, sid, id, context, removed)
			if err != nil {
				return nil, override, nil, err
			}
			items[sid] = item
			order = append(order, sid)
		}
	}
	owners := map[int64][]int64{}
	targets := map[int64]bool{}
	baseCount, faceCount := 0, 0
	var styles []MixedConicalAP242SurfaceStyle
	for _, id := range order {
		item := items[id]
		s := item.MixedConicalAP242SurfaceStyle
		owners[id] = append(owners[id], s.PresentationID)
		if item.base == 0 {
			s.BodyID = item.target
			baseCount++
		} else {
			base, ok := items[item.base]
			if !ok || base.base != 0 || base.PresentationID != item.PresentationID {
				return nil, override, nil, fail
			}
			owners[item.base] = append(owners[item.base], id)
			s.BodyID = base.target
			if item.contextID != 0 {
				if override.StyledItemID != 0 || item.target != s.BodyID || contexts[s.BodyID] == 0 {
					return nil, override, nil, fail
				}
				override, err = readMixedConicalDAGStyleContext(f, incoming, shuo, item, s.BodyID, contexts[s.BodyID], removed)
				if err != nil {
					return nil, override, nil, err
				}
				continue
			}
			s.FaceID = item.target
			faceCount++
			if faceBody[s.FaceID] != s.BodyID {
				return nil, override, nil, fail
			}
		}
		if contexts[s.BodyID] == 0 || item.presentationContext != contexts[s.BodyID] || targets[item.target] {
			return nil, override, nil, fail
		}
		targets[item.target] = true
		styles = append(styles, s)
	}
	if baseCount != 2 || faceCount == 0 || override.StyledItemID == 0 {
		return nil, override, nil, fail
	}
	for id, want := range owners {
		got := slices.Clone(incoming[id])
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			return nil, override, nil, fail
		}
	}
	for id := range removed {
		for _, owner := range incoming[id] {
			if !removed[owner] {
				return nil, override, nil, fail
			}
		}
	}
	view := cloneOvalFile(f)
	view.Order = make([]int64, 0, len(f.Order)-len(removed))
	for _, id := range f.Order {
		if removed[id] {
			delete(view.Entities, id)
		} else {
			view.Order = append(view.Order, id)
		}
	}
	raw, err := mappedPlanarSourceBytes(view)
	return styles, override, raw, err
}

func readMixedConicalDAGStyleItem(f *File, incoming map[int64][]int64, id, presentation, context int64, removed map[int64]bool) (mixedConicalDAGStyleItem, error) {
	var out mixedConicalDAGStyleItem
	fail := errMixedConicalAP242DAGAppearance
	r, err := simple(f, id, "STYLED_ITEM", 3)
	if err != nil {
		r, err = simple(f, id, "OVER_RIDING_STYLED_ITEM", 4)
		if err != nil {
			return out, err
		}
		out.base, err = ref(r.Args[3])
		if err != nil {
			return out, err
		}
	}
	if _, err := textValue(r.Args[0]); err != nil {
		return out, err
	}
	target, err := ref(r.Args[2])
	if err != nil {
		return out, err
	}
	ids, err := listRefs(r.Args[1], 1)
	if err != nil || len(ids) != 1 || !slices.Equal(incoming[ids[0]], []int64{id}) {
		return out, fail
	}
	assigned, err := simple(f, ids[0], "PRESENTATION_STYLE_ASSIGNMENT", 1)
	if err != nil {
		if out.base == 0 {
			return out, fail
		}
		assigned, err = simple(f, ids[0], "PRESENTATION_STYLE_BY_CONTEXT", 2)
		if err != nil {
			return out, err
		}
		out.contextID, err = ref(assigned.Args[1])
		if err != nil {
			return out, err
		}
	}
	fill, err := listRefs(assigned.Args[0], 1)
	if err != nil || len(fill) != 1 {
		return out, fail
	}
	color, chain, err := occurrenceSurfaceFill(f, fill[0])
	if err != nil {
		return out, err
	}
	owner := ids[0]
	for _, node := range chain {
		if removed[node] || !slices.Equal(incoming[node], []int64{owner}) {
			return out, fail
		}
		removed[node] = true
		owner = node
	}
	out.target, out.presentationContext = target, context
	out.MixedConicalAP242SurfaceStyle = MixedConicalAP242SurfaceStyle{
		StyledItemID: id, PresentationID: presentation, AssignmentID: ids[0], Color: color,
	}
	removed[id], removed[ids[0]] = true, true
	return out, nil
}

func readMixedConicalDAGStyleContext(f *File, incoming map[int64][]int64, source deepSHUOSource, item mixedConicalDAGStyleItem,
	bodyID, bodyContext int64, removed map[int64]bool) (OccurrenceSurfaceColor, error) {
	var zero OccurrenceSurfaceColor
	fail := errMixedConicalAP242DAGAppearance
	owner, err := ref(source.sdr.Args[1])
	if err != nil || owner != item.contextID {
		return zero, fail
	}
	ctx, err := simple(f, owner, "SHAPE_REPRESENTATION", 3)
	if err != nil || ctx.Args[2].Kind != Reference || ctx.Args[2].Ref != bodyContext {
		return zero, fail
	}
	if _, err := textValue(ctx.Args[0]); err != nil {
		return zero, err
	}
	placements, err := listRefs(ctx.Args[1], 1)
	if err != nil || len(placements) != 1 {
		return zero, fail
	}
	want := []int64{source.sdrID, item.AssignmentID}
	got := slices.Clone(incoming[owner])
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return zero, fail
	}
	if item.presentationContext != bodyContext {
		return zero, fail
	}
	removed[owner] = true
	return OccurrenceSurfaceColor{
		BodyID: bodyID, StyledItemID: item.StyledItemID, BaseStyledItemID: item.base, AssignmentID: item.AssignmentID,
		ContextRepresentationID: owner, DefinitionRepresentationID: source.sdrID, ProductShapeID: source.pdsID,
		PlacementID: placements[0], Color: item.Color,
	}, nil
}
