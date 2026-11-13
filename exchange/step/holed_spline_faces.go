package step

import (
	"errors"
	"io"
	"plainsolid/geom"
)

// HoledSplineFace is a positively oriented rectangular rational spline face
// with exactly one strictly interior clockwise rectangular hole.
type HoledSplineFace struct {
	Name        string
	Geometry    *geom.BSplineSurface
	Outer, Hole geom.ParameterRectangle
}
type HoledSplineFaceModel struct {
	Faces      []HoledSplineFace
	LengthUnit float64
}

func strictSplineHole(outer, hole geom.ParameterRectangle) bool {
	return outer.UMin < hole.UMin && hole.UMin < hole.UMax && hole.UMax < outer.UMax && outer.VMin < hole.VMin && hole.VMin < hole.VMax && hole.VMax < outer.VMax
}

// splineBoundView isolates one source boundary for the existing exact rectangle
// proof. For an inner loop it reverses traversal, never model or UV geometry.
func splineBoundView(f *File, fid, bid int64, reverse bool) (*File, error) {
	return splinePolygonBoundView(f, fid, bid, reverse, 4)
}

func splinePolygonBoundView(f *File, fid, bid int64, reverse bool, maxEdges int) (*File, error) {
	fail := func() (*File, error) { return nil, errors.New("step: invalid spline face boundary") }
	face, err := simple(f, fid, "ADVANCED_FACE", 4)
	if err != nil {
		return fail()
	}
	ent, ok := f.Entities[bid]
	if !ok || len(ent.Records) != 1 {
		return fail()
	}
	kind := ent.Records[0].Type
	if kind != "FACE_BOUND" && kind != "FACE_OUTER_BOUND" || reverse && kind == "FACE_OUTER_BOUND" {
		return fail()
	}
	bound, err := simple(f, bid, kind, 3)
	if err != nil {
		return fail()
	}
	boundSense, err := boolean(bound.Args[2])
	if err != nil {
		return fail()
	}
	lid, err := ref(bound.Args[1])
	if err != nil {
		return fail()
	}
	view := *f
	view.Entities = map[int64]Entity{}
	face.Args = append([]Value(nil), face.Args...)
	face.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: bid}}}
	view.Entities[fid] = Entity{ID: fid, Records: []Record{face}}
	// Copy only this boundary's referenced closure. Unrelated products and
	// metadata must not multiply per-loop memory and proof preparation cost.
	todo := []int64{fid}
	seen := map[int64]bool{}
	var refs func(Value)
	refs = func(v Value) {
		if v.Kind == Reference {
			todo = append(todo, v.Ref)
		}
		for _, x := range v.Items {
			refs(x)
		}
	}
	for len(todo) > 0 {
		id := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		entity, ok := view.Entities[id]
		if !ok {
			entity, ok = f.Entities[id]
			if !ok {
				return fail()
			}
			view.Entities[id] = entity
		}
		for _, r := range entity.Records {
			for _, v := range r.Args {
				refs(v)
			}
		}
	}
	bound.Args = append([]Value(nil), bound.Args...)
	bound.Args[2] = Value{Kind: Enumeration, Text: "T"}
	view.Entities[bid] = Entity{ID: bid, Records: []Record{bound}}
	if reverse != !boundSense {
		loop, err := simple(f, lid, "EDGE_LOOP", 2)
		if err != nil {
			return fail()
		}
		ids, err := listRefs(loop.Args[1], 4)
		if err != nil || len(ids) < 4 || len(ids) > maxEdges {
			return fail()
		}
		refs := make([]Value, len(ids))
		for i, id := range ids {
			use, err := simple(f, id, "ORIENTED_EDGE", 5)
			if err != nil {
				return fail()
			}
			sense, err := boolean(use.Args[4])
			if err != nil {
				return fail()
			}
			use.Args = append([]Value(nil), use.Args...)
			s := "T"
			if sense {
				s = "F"
			}
			use.Args[4] = Value{Kind: Enumeration, Text: s}
			view.Entities[id] = Entity{ID: id, Records: []Record{use}}
			refs[len(ids)-1-i] = Value{Kind: Reference, Ref: id}
		}
		loop.Args = append([]Value(nil), loop.Args...)
		loop.Args[1] = Value{Kind: Aggregate, Items: refs}
		view.Entities[lid] = Entity{ID: lid, Records: []Record{loop}}
	}
	return &view, nil
}
func decodeHoledSplineFace(f *File, id int64, maxPoints int) (HoledSplineFace, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return HoledSplineFace{}, e
	}
	bounds, e := listRefs(face.Args[1], 2)
	if e != nil || len(bounds) != 2 {
		return HoledSplineFace{}, errors.New("step: single-hole spline face requires exactly two bounds")
	}
	multi, e := decodeMultiHoledSplineFace(f, id, maxPoints)
	if e != nil {
		return HoledSplineFace{}, e
	}
	return HoledSplineFace{Name: multi.Name, Geometry: multi.Geometry, Outer: multi.Outer, Hole: multi.Holes[0]}, nil
}

// ReadHoledSplineFaces reads one-face open-shell surface models with exactly
// one rectangular hole per face. It proves each iso/model identity, strict UV
// containment, opposite winding and outer-patch injectivity. General trims,
// assemblies, negative face sense and multi-face shells remain unsupported.
func ReadHoledSplineFaces(r io.Reader, l Limits) (HoledSplineFaceModel, error) {
	l, err := l.defaults()
	if err != nil {
		return HoledSplineFaceModel{}, err
	}
	f, err := Parse(r, l)
	if err != nil {
		return HoledSplineFaceModel{}, err
	}
	result := HoledSplineFaceModel{}
	m, err := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		face, e := decodeHoledSplineFace(f, id, maxPoints)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		result.Faces = append(result.Faces, face)
		return BoundedSplineFace{Name: face.Name, Geometry: face.Geometry, Domain: face.Outer}, nil
	}, 8)
	if err != nil {
		return HoledSplineFaceModel{}, err
	}
	result.LengthUnit = m.LengthUnit
	return result, nil
}

// WriteHoledSplineFaces retains each original support and emits paired exact
// model/UV edges for both loops. Unrepresentable coefficients or endpoints fail
// before output. Existing hole-free readers deliberately reject this graph.
func WriteHoledSplineFaces(w io.Writer, m HoledSplineFaceModel) error {
	base := BoundedSplineFaceModel{LengthUnit: m.LengthUnit}
	holes := make([][]geom.ParameterRectangle, len(m.Faces))
	for i, face := range m.Faces {
		if !strictSplineHole(face.Outer, face.Hole) {
			return errors.New("step: spline hole must be strictly interior")
		}
		base.Faces = append(base.Faces, BoundedSplineFace{Name: face.Name, Geometry: face.Geometry, Domain: face.Outer})
		holes[i] = []geom.ParameterRectangle{face.Hole}
	}
	return writeSplineRectangles(w, base, holes)
}
