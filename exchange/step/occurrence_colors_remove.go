package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"reflect"
)

// OccurrenceColorRemoval reports the removed original claims. If
// Appearance.Overrides is empty, its source is product-local and must be read
// with ReadColoredAssembly; the old contextual native format stays strict.
type OccurrenceColorRemoval struct {
	Appearance OccurrenceColoredAssembly
	Removed    []OccurrenceSurfaceColor
}

// readFlatAppearance selects by declared syntax, never by trying another
// decoder after failure. Product-local and contextual proofs remain distinct.
func readFlatAppearance(raw []byte, source *File, l Limits) (OccurrenceColoredAssembly, error) {
	var m OccurrenceColoredAssembly
	contextual := false
	for _, en := range source.Entities {
		for _, r := range en.Records {
			if r.Type == "PRESENTATION_STYLE_BY_CONTEXT" {
				contextual = true
			}
		}
	}
	var e error
	if contextual {
		m, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), l)
	} else {
		m.Assembly, e = ReadColoredAssembly(bytes.NewReader(raw), l)
	}
	if e != nil {
		return OccurrenceColoredAssembly{}, e
	}
	a := m.Assembly.Assembly
	if len(a.Roots) != 1 {
		return OccurrenceColoredAssembly{}, geom.ErrInvalid
	}
	for _, o := range a.Occurrences {
		if o.Parent != a.Roots[0] || len(m.Assembly.ProductBodies[o.Child]) == 0 {
			return OccurrenceColoredAssembly{}, geom.ErrInvalid
		}
	}
	return m, nil
}

// RemoveOccurrenceSurfaceColor removes every equivalent claim for one existing
// flat NAUO. A known unstyled use is a validated no-op; an unknown ID is rejected.
// Shared fill dependencies, other overrides and original geometry are retained.
// The supplied source is never mutated and failures return no partial model.
func RemoveOccurrenceSurfaceColor(ctx context.Context, source *File, occurrenceID int64, l Limits) (OccurrenceColorRemoval, error) {
	var zero OccurrenceColorRemoval
	if ctx == nil || !independentNativeSourceRecordBound(source, 4096, 8) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return zero, e
	}
	before, e := readFlatAppearance(raw, source, l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	found := false
	for _, o := range before.Assembly.Assembly.Occurrences {
		if o.ID == occurrenceID {
			found = true
		}
	}
	if !found {
		return zero, geom.ErrInvalid
	}
	var removed, remaining []OccurrenceSurfaceColor
	candidates := map[int64]bool{}
	chain := map[int64]bool{}
	items := map[int64]bool{}
	for _, c := range before.Overrides {
		if c.OccurrenceID != occurrenceID {
			remaining = append(remaining, c)
			continue
		}
		removed = append(removed, c)
		for _, id := range []int64{c.StyledItemID, c.AssignmentID, c.ContextRepresentationID, c.DefinitionRepresentationID} {
			candidates[id] = true
		}
		items[c.StyledItemID] = true
		r, e := simple(source, c.AssignmentID, "PRESENTATION_STYLE_BY_CONTEXT", 2)
		if e != nil {
			return zero, e
		}
		ids, e := listRefs(r.Args[0], 1)
		if e != nil || len(ids) != 1 {
			return zero, geom.ErrInvalid
		}
		_, ids, e = occurrenceSurfaceFill(source, ids[0])
		if e != nil {
			return zero, e
		}
		for _, id := range ids {
			candidates[id] = true
			chain[id] = true
		}
	}
	if len(removed) == 0 {
		return OccurrenceColorRemoval{Appearance: before}, nil
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	for id, en := range view.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION" {
			continue
		}
		r := en.Records[0]
		if len(r.Args) != 3 {
			return zero, geom.ErrInvalid
		}
		ids, e := listRefs(r.Args[1], 1)
		if e != nil {
			return zero, e
		}
		var kept []Value
		changed := false
		for _, item := range ids {
			if items[item] {
				changed = true
			} else {
				kept = append(kept, Value{Kind: Reference, Ref: item})
			}
		}
		if changed {
			if len(kept) == 0 {
				return zero, geom.ErrInvalid
			}
			r.Args[1] = Value{Kind: Aggregate, Items: kept}
			view.Entities[id] = Entity{ID: id, Records: []Record{r}}
		}
	}
	refs := func(records []Record) []int64 {
		var out []int64
		var walk func(Value)
		walk = func(v Value) {
			if v.Kind == Reference {
				out = append(out, v.Ref)
			}
			for _, q := range v.Items {
				walk(q)
			}
		}
		for _, r := range records {
			for _, v := range r.Args {
				walk(v)
			}
		}
		return out
	}
	retained := map[int64]bool{}
	var queue []int64
	mark := func(id int64) error {
		if !candidates[id] {
			return nil
		}
		if !chain[id] {
			return geom.ErrInvalid
		}
		if !retained[id] {
			retained[id] = true
			queue = append(queue, id)
		}
		return nil
	}
	for owner, en := range view.Entities {
		if candidates[owner] {
			continue
		}
		for _, id := range refs(en.Records) {
			if e = mark(id); e != nil {
				return zero, e
			}
		}
	}
	for _, id := range refs(view.Header) {
		if e = mark(id); e != nil {
			return zero, e
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, id := range refs(view.Entities[queue[i]].Records) {
			if e = mark(id); e != nil {
				return zero, e
			}
		}
	}
	var order []int64
	for _, id := range view.Order {
		if candidates[id] && !retained[id] {
			delete(view.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	view.Order = order
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	edited, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	after, e := readFlatAppearance(edited, view, l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	// Compare the original model after precisely deleting the requested claims
	// with the independently re-proved default/contextual result.
	before.Overrides = remaining
	expected, e := occurrenceAppearanceClaims(ctx, before)
	if e != nil {
		return zero, e
	}
	actual, e := occurrenceAppearanceClaims(ctx, after)
	if e != nil {
		return zero, e
	}
	expected.Source = ""
	actual.Source = ""
	if !reflect.DeepEqual(expected, actual) || !reflect.DeepEqual(remaining, after.Overrides) {
		return zero, geom.ErrInvalid
	}
	return OccurrenceColorRemoval{Appearance: after, Removed: removed}, nil
}

// WriteFlatColoredAssemblySource re-proves and exports a flat product-local or
// contextual appearance source. It does not infer or persist an edited result.
func WriteFlatColoredAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(source, 4096, 8) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = readFlatAppearance(raw, source, l); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
