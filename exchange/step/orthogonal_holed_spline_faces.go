package step

import (
	"errors"
	"io"
	"plainsolid/geom"
)

// OrthogonalHoledSplineFace retains a counterclockwise outer polygon and
// counterclockwise hole polygons in source UV coordinates. STEP interior loops
// have the opposite physical traversal. Polygons may be concave, but must be
// simple and strictly separated. At least one hole is required.
type OrthogonalHoledSplineFace struct {
	Name     string
	Geometry *geom.BSplineSurface
	Boundary []geom.Vec2
	Holes    [][]geom.Vec2
}
type OrthogonalHoledSplineFaceModel struct {
	Faces      []OrthogonalHoledSplineFace
	LengthUnit float64
}

func orthogonalLoopsTouch(a, b []geom.Vec2) bool {
	for i, p := range a {
		q := a[(i+1)%len(a)]
		for j, r := range b {
			s := b[(j+1)%len(b)]
			if max(min(p.X, q.X), min(r.X, s.X)) <= min(max(p.X, q.X), max(r.X, s.X)) && max(min(p.Y, q.Y), min(r.Y, s.Y)) <= min(max(p.Y, q.Y), max(r.Y, s.Y)) {
				return true
			}
		}
	}
	return false
}

// With boundary contacts excluded, axis-aligned ray crossing needs only exact
// ordering of represented coordinates, with a half-open vertex convention.
func insideOrthogonal(p geom.Vec2, polygon []geom.Vec2) bool {
	inside := false
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		if (a.Y > p.Y) != (b.Y > p.Y) && a.X > p.X {
			inside = !inside
		}
	}
	return inside
}
func validateOrthogonalHoles(outer []geom.Vec2, holes [][]geom.Vec2) error {
	if _, e := orthogonalPolygon(outer); e != nil {
		return e
	}
	if len(holes) < 1 || len(holes) > MaxSplineFaceHoles {
		return errors.New("step: orthogonal hole count")
	}
	for i, h := range holes {
		if _, e := orthogonalPolygon(h); e != nil {
			return e
		}
		if orthogonalLoopsTouch(outer, h) || !insideOrthogonal(h[0], outer) {
			return errors.New("step: orthogonal hole outside or touching outer boundary")
		}
		for _, other := range holes[:i] {
			if orthogonalLoopsTouch(h, other) || insideOrthogonal(h[0], other) || insideOrthogonal(other[0], h) {
				return errors.New("step: orthogonal holes overlap, touch or nest")
			}
		}
	}
	return nil
}

func decodeOrthogonalHoledSplineFace(f *File, id int64, maxPoints int) (OrthogonalHoledSplineFace, int, error) {
	fail := func() (OrthogonalHoledSplineFace, int, error) {
		return OrthogonalHoledSplineFace{}, 0, errors.New("step: invalid orthogonal holed face")
	}
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	bounds, e := listRefs(face.Args[1], 2)
	if e != nil || len(bounds) < 2 || len(bounds) > 1+MaxSplineFaceHoles {
		return fail()
	}
	result := OrthogonalHoledSplineFace{}
	outer := false
	seen := map[int64]bool{}
	uses := 0
	for _, bid := range bounds {
		if seen[bid] {
			return fail()
		}
		seen[bid] = true
		view, e := splinePolygonBoundView(f, id, bid, false, MaxOrthogonalSplineVertices)
		if e != nil {
			return fail()
		}
		decoded, e := decodeOrthogonalSplineFace(view, id, maxPoints)
		if e == nil {
			if outer {
				return fail()
			}
			outer = true
			result.Name = decoded.Name
			result.Geometry = decoded.Geometry
			result.Boundary = decoded.Boundary
		} else {
			view, e = splinePolygonBoundView(f, id, bid, true, MaxOrthogonalSplineVertices)
			if e != nil {
				return fail()
			}
			decoded, e = decodeOrthogonalSplineFace(view, id, maxPoints)
			if e != nil {
				return fail()
			}
			result.Holes = append(result.Holes, decoded.Boundary)
		}
		uses += len(decoded.Boundary)
	}
	if !outer {
		return fail()
	}
	if e := validateOrthogonalHoles(result.Boundary, result.Holes); e != nil {
		return fail()
	}
	return result, uses, nil
}

// ReadOrthogonalHoledSplineFaces certifies each exact model/UV loop and proves
// strict containment on the injective support. It does not infer shell closure.
func ReadOrthogonalHoledSplineFaces(r io.Reader, l Limits) (OrthogonalHoledSplineFaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return OrthogonalHoledSplineFaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return OrthogonalHoledSplineFaceModel{}, e
	}
	out := OrthogonalHoledSplineFaceModel{}
	remaining := l.MaxEdgeUses
	m, e := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		bounds, e := listRefs(face.Args[1], 2)
		if e != nil || len(bounds) > 1+MaxSplineFaceHoles {
			return BoundedSplineFace{}, errors.New("step: orthogonal boundary limit")
		}
		count := 0
		for _, bid := range bounds {
			view, e := splinePolygonBoundView(f, id, bid, false, MaxOrthogonalSplineVertices)
			if e != nil {
				return BoundedSplineFace{}, e
			}
			n, e := orthogonalUseCount(view, id)
			if e != nil {
				return BoundedSplineFace{}, e
			}
			count += n
		}
		if count > remaining {
			return BoundedSplineFace{}, errors.New("step: orthogonal edge-use limit")
		}
		decoded, _, e := decodeOrthogonalHoledSplineFace(f, id, maxPoints)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		remaining -= count
		out.Faces = append(out.Faces, decoded)
		d, _ := orthogonalPolygon(decoded.Boundary)
		return BoundedSplineFace{Name: decoded.Name, Geometry: decoded.Geometry, Domain: d}, nil
	}, 0)
	if e != nil {
		return OrthogonalHoledSplineFaceModel{}, e
	}
	out.LengthUnit = m.LengthUnit
	return out, nil
}
