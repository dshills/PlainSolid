package step

import (
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
)

// OCCTHierarchyMapping records original ITEM_DEFINED_TRANSFORMATION/frame IDs
// and the separately derived neutral/effective frame IDs in normalized source.
// Transform is inverse(from)*to, in the explicitly selected target SI unit.
type OCCTHierarchyMapping struct {
	TransformationID, FromFrameID, ToFrameID int64
	DerivedFromFrameID, DerivedToFrameID     int64
	Transform                                geom.Affine
}

// OCCTNormalizedAnalyticHierarchy is explicitly the qualified pinned OCCT
// transfer convention, not a widening of the neutral-child normalized reader.
type OCCTNormalizedAnalyticHierarchy struct {
	Normalized NormalizedIndependentAnalyticHierarchy
	Mappings   []OCCTHierarchyMapping
}

// ReadOCCTNormalizedAnalyticHierarchy retains original mixed SI source and
// derives inverse(from)*to after scaling each frame in its owning context.
// Proper signed-coordinate original frames, bounded graph and exact binary64
// derived translations are mandatory; no geometry or frame snapping occurs.
func ReadOCCTNormalizedAnalyticHierarchy(ctx context.Context, r io.Reader, target IndependentMaterialSIUnit, l Limits, tol float64) (OCCTNormalizedAnalyticHierarchy, error) {
	var mappings []OCCTHierarchyMapping
	m, e := readNormalizedAnalyticHierarchy(ctx, r, target, l, tol, true, &mappings)
	if e != nil {
		return OCCTNormalizedAnalyticHierarchy{}, e
	}
	return OCCTNormalizedAnalyticHierarchy{Normalized: m, Mappings: mappings}, nil
}
func normalizeOCCTHierarchyFrames(ctx context.Context, f *File, l Limits) ([]OCCTHierarchyMapping, error) {
	bad := func() ([]OCCTHierarchyMapping, error) { return nil, geom.ErrInvalid }
	next := int64(0)
	for id := range f.Entities {
		if id > next {
			next = id
		}
	}
	if next > math.MaxInt64-512 {
		return bad()
	}
	add := func(kind string, args ...Value) int64 {
		next++
		f.Order = append(f.Order, next)
		f.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
		return next
	}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	var encodeError error
	vec := func(kind string, a [3]float64) int64 {
		v := []Value{}
		for _, x := range a {
			text, e := extrusionDirectionDecimal(new(big.Rat).SetFloat64(x))
			if e != nil {
				encodeError = e
			}
			v = append(v, Value{Kind: Number, Text: text})
		}
		return add(kind, Value{Kind: String}, Value{Kind: Aggregate, Items: v})
	}
	frame := func(a geom.Affine) int64 {
		p := vec("CARTESIAN_POINT", [3]float64{a[0][3], a[1][3], a[2][3]})
		z := vec("DIRECTION", [3]float64{a[0][2], a[1][2], a[2][2]})
		x := vec("DIRECTION", [3]float64{a[0][0], a[1][0], a[2][0]})
		return add("AXIS2_PLACEMENT_3D", Value{Kind: String}, refv(p), refv(z), refv(x))
	}
	member := func(rep, id int64) bool {
		en := f.Entities[rep]
		if len(en.Records) != 1 {
			return false
		}
		r := en.Records[0]
		if (r.Type != "SHAPE_REPRESENTATION" && r.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION") || len(r.Args) != 3 || r.Args[1].Kind != Aggregate {
			return false
		}
		for _, v := range r.Args[1].Items {
			if v.Kind == Reference && v.Ref == id {
				return true
			}
		}
		return false
	}
	appendFrame := func(rep, id int64) {
		en := f.Entities[rep]
		en.Records[0].Args[1].Items = append(en.Records[0].Args[1].Items, refv(id))
		f.Entities[rep] = en
	}
	ids := append([]int64(nil), f.Order...)
	seen := map[int64]bool{}
	out := []OCCTHierarchyMapping{}
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		en := f.Entities[id]
		var rr, rt *Record
		for j := range en.Records {
			r := &en.Records[j]
			if r.Type == "REPRESENTATION_RELATIONSHIP" {
				rr = r
			}
			if r.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
				rt = r
			}
		}
		if rt == nil {
			continue
		}
		if rr == nil || len(rr.Args) != 4 || len(rt.Args) != 1 || rt.Args[0].Kind != Reference || rr.Args[2].Kind != Reference || rr.Args[3].Kind != Reference || len(out) >= 64 {
			return bad()
		}
		tid := rt.Args[0].Ref
		if seen[tid] {
			return bad()
		}
		seen[tid] = true
		tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil || tr.Args[2].Kind != Reference || tr.Args[3].Kind != Reference {
			return bad()
		}
		fromID, toID := tr.Args[2].Ref, tr.Args[3].Ref
		child, parent := rr.Args[2].Ref, rr.Args[3].Ref
		if !member(child, fromID) || !member(parent, toID) {
			return bad()
		}
		from, e := mappedRationalFrame(f, fromID)
		if e != nil {
			return bad()
		}
		to, e := mappedRationalFrame(f, toID)
		if e != nil {
			return bad()
		}
		if _, e = independentTorusAssemblyPose(from); e != nil {
			return bad()
		}
		if _, e = independentTorusAssemblyPose(to); e != nil {
			return bad()
		}
		a, e := independentTorusAssemblyPose(composeMappedPose(inverseMappedPose(from), to))
		if e != nil {
			return bad()
		}
		if len(f.Entities) > l.MaxEntities-8 {
			return bad()
		}
		neutral := frame(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
		target := frame(a)
		if encodeError != nil {
			return nil, encodeError
		}
		appendFrame(child, neutral)
		appendFrame(parent, target)
		te := f.Entities[tid]
		te.Records[0].Args[2] = refv(neutral)
		te.Records[0].Args[3] = refv(target)
		f.Entities[tid] = te
		out = append(out, OCCTHierarchyMapping{tid, fromID, toID, neutral, target, a})
	}
	if len(out) == 0 {
		return bad()
	}
	return out, nil
}
