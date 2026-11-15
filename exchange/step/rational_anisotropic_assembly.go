package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"plainsolid/geom"
)

// rationalAnisotropicProofView retains original source and replaces only
// disjoint representation-frame dependencies in a private ownership proof view.
func rationalAnisotropicProofView(ctx context.Context, r io.Reader, l Limits) (*File, []byte, map[int64]mappedRationalBasis, error) {
	bad := func() (*File, []byte, map[int64]mappedRationalBasis, error) { return nil, nil, nil, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return nil, nil, nil, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	source, e := Parse(r, l)
	if e != nil {
		return nil, nil, nil, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return bad()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	material := map[int64]bool{}
	for id, en := range source.Entities {
		if e = ctx.Err(); e != nil {
			return nil, nil, nil, e
		}
		if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			c, e := nauoClosure(source, []int64{id}, l.MaxDepth)
			if e != nil {
				return bad()
			}
			for k := range c {
				material[k] = true
			}
		}
	}
	frames := map[int64]mappedRationalBasis{}
	removed := map[int64]bool{}
	for _, id := range source.Order {
		if e = ctx.Err(); e != nil {
			return nil, nil, nil, e
		}
		en := source.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		if rec.Type != "SHAPE_REPRESENTATION" && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		if len(rec.Args) != 3 {
			return bad()
		}
		items, e := listRefs(rec.Args[1], 1)
		if e != nil {
			return bad()
		}
		for _, fr := range items {
			en := source.Entities[fr]
			if len(en.Records) != 1 || en.Records[0].Type != "AXIS2_PLACEMENT_3D" {
				continue
			}
			b, e := mappedRationalFrame(source, fr)
			if e != nil || rationalHierarchyFrameBound(b) != nil {
				return bad()
			}
			frames[fr] = b
			c, e := nauoClosure(source, []int64{fr}, l.MaxDepth)
			if e != nil {
				return bad()
			}
			for key := range c {
				if material[key] {
					return bad()
				}
				if key == fr {
					continue
				}
				dep := source.Entities[key]
				if len(dep.Records) != 1 || len(dep.Records[0].Args) != 2 || (dep.Records[0].Type != "CARTESIAN_POINT" && dep.Records[0].Type != "DIRECTION") {
					return bad()
				}
				if _, e = textValue(dep.Records[0].Args[0]); e != nil {
					return bad()
				}
				removed[key] = true
			}
		}
	}
	if len(frames) == 0 || len(frames) > 256 {
		return bad()
	}
	next := int64(0)
	for id := range source.Entities {
		if id > next {
			next = id
		}
	}
	if next > math.MaxInt64-3 {
		return bad()
	}
	// Only original frame dependencies disjoint from every body are removed from
	// this private view. Original source remains byte-value authoritative.
	for id := range removed {
		delete(view.Entities, id)
	}
	order := []int64{}
	for _, id := range view.Order {
		if !removed[id] {
			order = append(order, id)
		}
	}
	view.Order = order
	add := func(kind string, v [3]string) int64 {
		next++
		view.Order = append(view.Order, next)
		items := []Value{}
		for _, s := range v {
			items = append(items, Value{Kind: Number, Text: s})
		}
		view.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: []Value{{Kind: String}, {Kind: Aggregate, Items: items}}}}}
		return next
	}
	point := add("CARTESIAN_POINT", [3]string{"0.", "0.", "0."})
	z := add("DIRECTION", [3]string{"0.", "0.", "1."})
	x := add("DIRECTION", [3]string{"1.", "0.", "0."})
	for id := range frames {
		en := view.Entities[id]
		en.Records[0].Args[1] = Value{Kind: Reference, Ref: point}
		en.Records[0].Args[2] = Value{Kind: Reference, Ref: z}
		en.Records[0].Args[3] = Value{Kind: Reference, Ref: x}
		view.Entities[id] = en
	}
	b, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return bad()
	}
	return source, b, frames, nil
}

// RationalWrappedAnisotropicStationAssembly preserves one original local
// material and immutable exact rational occurrence frames. No rounded world
// transform or baked world solid is supplied. Source child frames are neutral;
// all products share metre or millimetre units and the flat 2..16-use grammar.
type RationalWrappedAnisotropicStationAssembly struct {
	Source        *File
	Definitions   []AssemblyProduct
	Roots         []int64
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Body          WrappedAnisotropicStationLoftBody
	Occurrences   []ExactNAUOOccurrence
}

func ReadRationalWrappedAnisotropicStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (RationalWrappedAnisotropicStationAssembly, error) {
	var zero RationalWrappedAnisotropicStationAssembly
	source, view, frames, e := rationalAnisotropicProofView(ctx, r, l)
	if e != nil {
		return zero, e
	}
	proof, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(view), l, o)
	if e != nil {
		return zero, e
	}
	out := RationalWrappedAnisotropicStationAssembly{Source: source, Definitions: proof.Assembly.Products, Roots: proof.Assembly.Roots, Product: proof.Product, SourceBodyID: proof.SourceBodyID, SourceFaceIDs: proof.SourceFaceIDs, Body: proof.Body}
	for _, u := range proof.Occurrences {
		from, ok := frames[u.FromPlacementID]
		if !ok {
			return zero, geom.ErrInvalid
		}
		neutral, e := independentTorusAssemblyPose(from)
		if e != nil || neutral != geom.Identity() {
			return zero, geom.ErrInvalid
		}
		to, ok := frames[u.ToPlacementID]
		if !ok {
			return zero, geom.ErrInvalid
		}
		u.pose = to
		out.Occurrences = append(out.Occurrences, u)
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// WriteRationalWrappedAnisotropicStationAssemblySource re-proves source and
// exports its actual standard STEP placements, never the private neutral view.
func WriteRationalWrappedAnisotropicStationAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(source, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), l, o); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
