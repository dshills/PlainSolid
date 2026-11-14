package step

import (
	"bytes"
	"errors"
	"io"
)

// OccurrenceSurfaceColor binds a whole-body override to one explicit NAUO
// use. Duplicate equal claims retain their separate original record identities.
type OccurrenceSurfaceColor struct {
	OccurrenceID, BodyID, StyledItemID, BaseStyledItemID              int64
	AssignmentID, ContextRepresentationID, DefinitionRepresentationID int64
	ProductShapeID, CDSRID, TransformationID, PlacementID             int64
	Color                                                             RGB
}

// OccurrenceColoredAssembly retains product-local styles and separately selected
// occurrence overrides. Its Assembly.Source is the complete original source.
// This is a flat, planar, whole-body fill subset, not general contextual styling.
type OccurrenceColoredAssembly struct {
	Assembly  ColoredAssembly
	Overrides []OccurrenceSurfaceColor
}

// ReadOccurrenceColoredAssembly authenticates XCAF's explicit contextual style
// representation against the occurrence's PDS/CDSR/IDT and target local body.
// The older product-local reader deliberately continues rejecting this syntax.
func ReadOccurrenceColoredAssembly(r io.Reader, l Limits) (OccurrenceColoredAssembly, error) {
	return readOccurrenceColoredAssembly(r, l, true)
}

// The non-flat private branch authenticates identical style ownership chains;
// its caller must independently validate the complete supported hierarchy.
func readOccurrenceColoredAssembly(r io.Reader, l Limits, flat bool) (OccurrenceColoredAssembly, error) {
	fail := func() (OccurrenceColoredAssembly, error) {
		return OccurrenceColoredAssembly{}, errors.New("step: unsupported or inconsistent occurrence surface style")
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return OccurrenceColoredAssembly{}, e
	}
	if r == nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return OccurrenceColoredAssembly{}, e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return fail()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(owner int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], owner)
		}
		for _, q := range v.Items {
			walk(owner, q)
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				walk(id, v)
			}
		}
	}
	one := func(v Value) (int64, error) {
		ids, e := listRefs(v, 1)
		if e != nil || len(ids) != 1 {
			return 0, errors.New("step: expected one style item")
		}
		return ids[0], nil
	}
	remove := map[int64]bool{}
	chain := map[int64]bool{}
	overrideIDs := map[int64]bool{}
	var claims []OccurrenceSurfaceColor
	_, bodyContexts, e := colorBodyIDs(f)
	if e != nil {
		return fail()
	}
	for _, aid := range f.Order {
		en := f.Entities[aid]
		if len(en.Records) != 1 || en.Records[0].Type != "PRESENTATION_STYLE_BY_CONTEXT" {
			continue
		}
		as, e := simple(f, aid, "PRESENTATION_STYLE_BY_CONTEXT", 2)
		if e != nil {
			return fail()
		}
		if len(incoming[aid]) != 1 {
			return fail()
		}
		oid := incoming[aid][0]
		over, e := simple(f, oid, "OVER_RIDING_STYLED_ITEM", 4)
		if e != nil {
			return fail()
		}
		if _, e = textValue(over.Args[0]); e != nil {
			return fail()
		}
		assigned, e := one(over.Args[1])
		if e != nil || assigned != aid {
			return fail()
		}
		body, e := ref(over.Args[2])
		if e != nil || bodyContexts[body] == 0 {
			return fail()
		}
		parent, e := ref(over.Args[3])
		if e != nil {
			return fail()
		}
		base, e := simple(f, parent, "STYLED_ITEM", 3)
		if e != nil || base.Args[2].Kind != Reference || base.Args[2].Ref != body {
			return fail()
		}
		if len(incoming[oid]) != 1 {
			return fail()
		}
		presentation, e := simple(f, incoming[oid][0], "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION", 3)
		if e != nil || presentation.Args[2].Kind != Reference || presentation.Args[2].Ref != bodyContexts[body] {
			return fail()
		}
		cid, e := ref(as.Args[1])
		if e != nil {
			return fail()
		}
		context, e := simple(f, cid, "SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		if _, e = textValue(context.Args[0]); e != nil {
			return fail()
		}
		placement, e := one(context.Args[1])
		if e != nil || context.Args[2].Kind != Reference || context.Args[2].Ref != bodyContexts[body] {
			return fail()
		}
		var sdrID, pdsID int64
		for _, owner := range incoming[cid] {
			if owner == aid {
				continue
			}
			sdr, e := simple(f, owner, "SHAPE_DEFINITION_REPRESENTATION", 2)
			if e != nil || sdrID != 0 || sdr.Args[1].Kind != Reference || sdr.Args[1].Ref != cid {
				return fail()
			}
			sdrID = owner
			pdsID, e = ref(sdr.Args[0])
			if e != nil {
				return fail()
			}
		}
		if sdrID == 0 {
			return fail()
		}
		pds, e := simple(f, pdsID, "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil {
			return fail()
		}
		nauo, e := ref(pds.Args[2])
		if e != nil {
			return fail()
		}
		if _, e = simple(f, nauo, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6); e != nil {
			return fail()
		}
		var cdsrID, tid int64
		for _, id := range incoming[pdsID] {
			cd, e := simple(f, id, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", 2)
			if e != nil {
				continue
			}
			if cdsrID != 0 || cd.Args[1].Kind != Reference || cd.Args[1].Ref != pdsID {
				return fail()
			}
			cdsrID = id
			rel, e := ref(cd.Args[0])
			if e != nil {
				return fail()
			}
			for _, q := range f.Entities[rel].Records {
				if q.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
					if len(q.Args) != 1 || tid != 0 {
						return fail()
					}
					tid, e = ref(q.Args[0])
					if e != nil {
						return fail()
					}
				}
			}
		}
		tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil || cdsrID == 0 || tr.Args[3].Kind != Reference || tr.Args[3].Ref != placement {
			return fail()
		}
		usage, e := one(as.Args[0])
		if e != nil {
			return fail()
		}
		color, ids, e := occurrenceSurfaceFill(f, usage)
		if e != nil {
			return fail()
		}
		for _, id := range ids {
			remove[id] = true
			chain[id] = true
		}
		for _, id := range []int64{oid, aid, cid, sdrID} {
			remove[id] = true
		}
		overrideIDs[oid] = true
		claims = append(claims, OccurrenceSurfaceColor{nauo, body, oid, parent, aid, cid, sdrID, pdsID, cdsrID, tid, placement, color})
	}
	if len(claims) == 0 || len(claims) > 64 {
		return fail()
	}
	// Remove only authenticated override items from their original presentations.
	for id, en := range view.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION" {
			continue
		}
		q := en.Records[0]
		if len(q.Args) != 3 {
			return fail()
		}
		items, e := listRefs(q.Args[1], 1)
		if e != nil {
			return fail()
		}
		var keep []Value
		for _, item := range items {
			if !overrideIDs[item] {
				keep = append(keep, Value{Kind: Reference, Ref: item})
			}
		}
		if len(keep) == 0 {
			return fail()
		}
		q.Args[1] = Value{Kind: Aggregate, Items: keep}
		view.Entities[id] = Entity{ID: id, Records: []Record{q}}
	}
	// Shared fill records may remain only when the old strict style proof owns
	// them. Structural context records cannot be borrowed by any other graph.
	for changed := true; changed; {
		changed = false
		for owner, en := range view.Entities {
			if remove[owner] {
				continue
			}
			var refs []int64
			var collect func(Value)
			collect = func(v Value) {
				if v.Kind == Reference {
					refs = append(refs, v.Ref)
				}
				for _, q := range v.Items {
					collect(q)
				}
			}
			for _, rec := range en.Records {
				for _, v := range rec.Args {
					collect(v)
				}
			}
			for _, id := range refs {
				if !remove[id] {
					continue
				}
				if !chain[id] {
					return fail()
				}
				delete(remove, id)
				changed = true
			}
		}
	}
	var order []int64
	for _, id := range view.Order {
		if remove[id] {
			delete(view.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	view.Order = order
	normalized, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return fail()
	}
	base, e := ReadColoredAssembly(bytes.NewReader(normalized), l)
	if e != nil {
		return OccurrenceColoredAssembly{}, e
	}
	if len(base.Assembly.Roots) != 1 {
		return fail()
	}
	root := base.Assembly.Roots[0]
	uses := map[int64]AssemblyOccurrence{}
	for _, o := range base.Assembly.Occurrences {
		if flat && (o.Parent != root || len(base.ProductBodies[o.Child]) == 0) {
			return fail()
		}
		uses[o.ID] = o
	}
	colors := map[int64]RGB{}
	for _, c := range claims {
		o, ok := uses[c.OccurrenceID]
		if !ok {
			return fail()
		}
		items, _, e := base.Assembly.productItems(o.Child)
		if e != nil || len(items) != 1 || items[0] != c.BodyID {
			return fail()
		}
		bodies := base.ProductBodies[o.Child]
		if len(bodies) != 1 || bodies[0].SurfaceColor == nil || len(bodies[0].FaceColors) != 0 {
			return fail()
		}
		if old, ok := colors[c.OccurrenceID]; ok && old != c.Color {
			return fail()
		}
		colors[c.OccurrenceID] = c.Color
	}
	base.Assembly.Source = f
	return OccurrenceColoredAssembly{base, claims}, nil
}

// WriteOccurrenceColoredAssemblySource validates and re-emits original source;
// it does not synthesize edits from mutable result color fields.
func WriteOccurrenceColoredAssemblySource(w io.Writer, f *File, l Limits) error {
	if w == nil || f == nil {
		return errors.New("step: nil occurrence color source")
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

func occurrenceSurfaceFill(f *File, id int64) (RGB, []int64, error) {
	fail := func() (RGB, []int64, error) { return RGB{}, nil, errors.New("step: unsupported occurrence fill") }
	var ids []int64
	take := func(kind string, n int) (Record, error) {
		r, e := simple(f, id, kind, n)
		if e == nil {
			ids = append(ids, id)
		}
		return r, e
	}
	one := func(v Value) (int64, error) {
		a, e := listRefs(v, 1)
		if e != nil || len(a) != 1 {
			return 0, errors.New("step: one fill required")
		}
		return a[0], nil
	}
	r, e := take("SURFACE_STYLE_USAGE", 2)
	if e != nil || r.Args[0].Kind != Enumeration || r.Args[0].Text != "BOTH" {
		return fail()
	}
	id, e = ref(r.Args[1])
	if e != nil {
		return fail()
	}
	r, e = take("SURFACE_SIDE_STYLE", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return fail()
	}
	id, e = one(r.Args[1])
	if e != nil {
		return fail()
	}
	r, e = take("SURFACE_STYLE_FILL_AREA", 1)
	if e != nil {
		return fail()
	}
	id, e = ref(r.Args[0])
	if e != nil {
		return fail()
	}
	r, e = take("FILL_AREA_STYLE", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return fail()
	}
	id, e = one(r.Args[1])
	if e != nil {
		return fail()
	}
	r, e = take("FILL_AREA_STYLE_COLOUR", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return fail()
	}
	id, e = ref(r.Args[1])
	if e != nil {
		return fail()
	}
	c, e := readSurfaceStyleColour(f, id)
	if e != nil {
		return fail()
	}
	return c, append(ids, id), nil
}

func occurrenceColorSourceLimits(l Limits) (Limits, error) {
	l, e := l.defaults()
	if e != nil {
		return Limits{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	l.MaxEdgeUses = min(l.MaxEdgeUses, 4096)
	l.MaxSolids = min(l.MaxSolids, 16)
	return l, nil
}
