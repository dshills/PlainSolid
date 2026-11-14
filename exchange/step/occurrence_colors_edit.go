package step

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"reflect"
)

// EditOccurrenceSurfaceColor changes an existing authenticated whole-body
// override selected by its original NAUO ID. It does not create a new override
// on an unstyled use, change a product default, or edit geometry/placement.
// All equal duplicate claims are updated together. Shared fill dependencies
// are copied before mutation; the supplied File is never modified.
func EditOccurrenceSurfaceColor(ctx context.Context, source *File, occurrenceID int64, color RGB, l Limits) (OccurrenceColoredAssembly, error) {
	var zero OccurrenceColoredAssembly
	if ctx == nil || !validRGB(color) || !independentNativeSourceRecordBound(source, 4096, 8) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return zero, e
	}
	before, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return editAuthenticatedOccurrenceSurfaceColor(ctx, source, raw, before, occurrenceID, color, l)
}

// Only called after the complete contextual source proof has succeeded.
func editAuthenticatedOccurrenceSurfaceColor(ctx context.Context, source *File, raw []byte, before OccurrenceColoredAssembly, occurrenceID int64, color RGB, l Limits) (OccurrenceColoredAssembly, error) {
	var zero OccurrenceColoredAssembly
	chain := map[int64]bool{}
	colors := map[int64]bool{}
	assignments := map[int64]int64{}
	same := true
	for _, claim := range before.Overrides {
		if claim.OccurrenceID != occurrenceID {
			continue
		}
		same = same && claim.Color == color
		a, e := simple(source, claim.AssignmentID, "PRESENTATION_STYLE_BY_CONTEXT", 2)
		if e != nil {
			return zero, e
		}
		uses, e := listRefs(a.Args[0], 1)
		if e != nil || len(uses) != 1 {
			return zero, geom.ErrInvalid
		}
		_, ids, e := occurrenceSurfaceFill(source, uses[0])
		if e != nil {
			return zero, e
		}
		for _, id := range ids {
			chain[id] = true
		}
		colors[ids[len(ids)-1]] = true
		assignments[claim.AssignmentID] = uses[0]
	}
	if len(assignments) == 0 {
		return zero, geom.ErrInvalid
	}
	if same {
		return before, nil
	}
	// Seed sharing from incoming references outside the selected assignments and
	// their complete fill chains, then propagate along every retained dependency.
	references := func(en Entity) []int64 {
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
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(v)
			}
		}
		return out
	}
	shared := map[int64]bool{}
	var queue []int64
	mark := func(id int64) {
		if chain[id] && !shared[id] {
			shared[id] = true
			queue = append(queue, id)
		}
	}
	for owner, en := range source.Entities {
		if chain[owner] {
			continue
		}
		if _, ok := assignments[owner]; ok {
			continue
		}
		for _, id := range references(en) {
			mark(id)
		}
	}
	for _, record := range source.Header {
		for _, id := range references(Entity{Records: []Record{record}}) {
			mark(id)
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, id := range references(source.Entities[queue[i]]) {
			mark(id)
		}
	}
	view, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		return zero, e
	}
	maxID := int64(0)
	for id := range source.Entities {
		maxID = max(maxID, id)
	}
	if maxID > math.MaxInt64-int64(len(shared)) {
		return zero, geom.ErrConvergence
	}
	replacement := map[int64]int64{}
	for _, id := range source.Order {
		if !chain[id] {
			continue
		}
		replacement[id] = id
		if shared[id] {
			maxID++
			replacement[id] = maxID
		}
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if v.Kind == Reference {
			if id, ok := replacement[v.Ref]; ok {
				v.Ref = id
			}
		}
		if v.Items != nil {
			items := make([]Value, len(v.Items))
			for i, q := range v.Items {
				items[i] = rewrite(q)
			}
			v.Items = items
		}
		return v
	}
	for _, id := range source.Order {
		if !chain[id] {
			continue
		}
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		en := source.Entities[id]
		out := Entity{ID: replacement[id]}
		for _, r := range en.Records {
			copy := Record{Type: r.Type, Args: make([]Value, len(r.Args))}
			for i, v := range r.Args {
				copy.Args[i] = rewrite(v)
			}
			out.Records = append(out.Records, copy)
		}
		if colors[id] {
			out.Records = []Record{{Type: "COLOUR_RGB", Args: []Value{{Kind: String}, {Kind: Number, Text: real(color.R)}, {Kind: Number, Text: real(color.G)}, {Kind: Number, Text: real(color.B)}}}}
		}
		view.Entities[out.ID] = out
		if out.ID != id {
			view.Order = append(view.Order, out.ID)
		}
	}
	for id, usage := range assignments {
		en := view.Entities[id]
		r := en.Records[0]
		r.Args[0] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: replacement[usage]}}}
		view.Entities[id] = Entity{ID: id, Records: []Record{r}}
	}
	edited, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	after, e := ReadOccurrenceColoredAssembly(bytes.NewReader(edited), l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	// Compare all regenerated geometry, styles and source roles after allowing
	// precisely the requested contextual color changes and the source encoding.
	expected, e := occurrenceColorsClaims(ctx, before)
	if e != nil {
		return zero, e
	}
	actual, e := occurrenceColorsClaims(ctx, after)
	if e != nil {
		return zero, e
	}
	expected.Source = ""
	actual.Source = ""
	for i := range expected.Overrides {
		if expected.Overrides[i].OccurrenceID == occurrenceID {
			expected.Overrides[i].Color = color
		}
	}
	if !reflect.DeepEqual(expected, actual) {
		return zero, geom.ErrInvalid
	}
	return after, nil
}
