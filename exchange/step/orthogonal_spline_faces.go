package step

import (
	"errors"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
)

const MaxOrthogonalSplineVertices = 64

// OrthogonalSplineFace retains a simple counterclockwise UV polygon composed
// of alternating constant-U/constant-V edges on its original spline support.
// Concave polygons are supported; holes and diagonal UV edges are not.
type OrthogonalSplineFace struct {
	Name     string
	Geometry *geom.BSplineSurface
	Boundary []geom.Vec2
}
type OrthogonalSplineFaceModel struct {
	Faces      []OrthogonalSplineFace
	LengthUnit float64
}

func orthogonalPolygon(p []geom.Vec2) (geom.ParameterRectangle, error) {
	fail := func() (geom.ParameterRectangle, error) {
		return geom.ParameterRectangle{}, errors.New("step: invalid orthogonal spline polygon")
	}
	if len(p) < 4 || len(p) > MaxOrthogonalSplineVertices || len(p)%2 != 0 {
		return fail()
	}
	for _, v := range p {
		if !finiteNumber(v.X) || !finiteNumber(v.Y) {
			return fail()
		}
	}
	d := geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}
	area := new(big.Rat)
	for i, a := range p {
		b, c := p[(i+1)%len(p)], p[(i+2)%len(p)]
		if !finiteNumber(a.X) || !finiteNumber(a.Y) || (a.X == b.X) == (a.Y == b.Y) || (a.X == b.X) == (b.X == c.X) {
			return fail()
		}
		d.UMin = min(d.UMin, a.X)
		d.UMax = max(d.UMax, a.X)
		d.VMin = min(d.VMin, a.Y)
		d.VMax = max(d.VMax, a.Y)
		rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(rat(a.X), rat(b.Y)), new(big.Rat).Mul(rat(a.Y), rat(b.X))))
		for j := i + 1; j < len(p); j++ {
			if j == i+1 || i == 0 && j == len(p)-1 {
				continue
			}
			u, v := p[j], p[(j+1)%len(p)]
			if max(min(a.X, b.X), min(u.X, v.X)) <= min(max(a.X, b.X), max(u.X, v.X)) && max(min(a.Y, b.Y), min(u.Y, v.Y)) <= min(max(a.Y, b.Y), max(u.Y, v.Y)) {
				return fail()
			}
		}
	}
	if area.Sign() != 1 {
		return fail()
	}
	return d, nil
}
func orthogonalBoundaries(s *geom.BSplineSurface, p []geom.Vec2) ([]*geom.ExactSplineCurve, error) {
	d, e := orthogonalPolygon(p)
	if e != nil {
		return nil, e
	}
	o := geom.SurfacePatchOptions{MaxOperations: 200000, MaxCoefficientBits: 4096}
	patch, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		return nil, e
	}
	patch, e = patch.Trim(d)
	if e != nil {
		return nil, e
	}
	if e = certifySplineFacePatch(patch); e != nil {
		return nil, e
	}
	curves := make([]*geom.ExactSplineCurve, len(p))
	for i, a := range p {
		b := p[(i+1)%len(p)]
		axis, fixed, lo, hi := 0, a.X, min(a.Y, b.Y), max(a.Y, b.Y)
		if a.Y == b.Y {
			axis, fixed, lo, hi = 1, a.Y, min(a.X, b.X), max(a.X, b.X)
		}
		c, e := patch.IsoCurve(axis, fixed, o)
		if e != nil {
			return nil, e
		}
		c, e = c.Restrict(lo, hi, o)
		if e != nil {
			return nil, e
		}
		curves[i] = c
	}
	return curves, nil
}
func orthogonalUseCount(f *File, id int64) (int, error) {
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return 0, e
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return 0, errors.New("step: orthogonal face requires one boundary")
	}
	ent := f.Entities[bounds[0]]
	if len(ent.Records) != 1 || len(ent.Records[0].Args) != 3 {
		return 0, errors.New("step: invalid orthogonal bound")
	}
	lid, e := ref(ent.Records[0].Args[1])
	if e != nil {
		return 0, e
	}
	loop, e := simple(f, lid, "EDGE_LOOP", 2)
	if e != nil {
		return 0, e
	}
	uses, e := listRefs(loop.Args[1], 4)
	if e != nil || len(uses) > MaxOrthogonalSplineVertices {
		return 0, errors.New("step: orthogonal edge count")
	}
	return len(uses), nil
}
func decodeOrthogonalSplineFace(f *File, id int64, maxPoints int) (OrthogonalSplineFace, error) {
	fail := func() (OrthogonalSplineFace, error) {
		return OrthogonalSplineFace{}, errors.New("step: unsupported orthogonal spline face graph")
	}
	n, e := orthogonalUseCount(f, id)
	if e != nil {
		return fail()
	}
	face, uses, e := analyticFaceLoop(f, id, n)
	if e != nil {
		return fail()
	}
	sense, e := boolean(face.Args[3])
	if e != nil || !sense {
		return fail()
	}
	name, e := textValue(face.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	if e := exactSplineWeightNormalization(f, sid, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
		return fail()
	}
	support, e := DecodeBSplineSurface(f, sid, maxPoints)
	if e != nil {
		return fail()
	}
	type edge struct {
		start, end int64
		axis       int
		fixed      float64
		curve      *geom.BSpline
	}
	edges := make([]edge, n)
	seen := map[int64]bool{}
	for i, use := range uses {
		if seen[use.id] {
			return fail()
		}
		seen[use.id] = true
		start, end, g, e := analyticEdge(f, use.id)
		if e != nil || start == end {
			return fail()
		}
		sc, e := simple(f, g, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
			return fail()
		}
		cid, e := ref(sc.Args[1])
		if e != nil {
			return fail()
		}
		if e := exactSplineWeightNormalization(f, cid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
			return fail()
		}
		curve, e := DecodeBSplineCurve(f, cid, maxPoints)
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 1)
		if e != nil || len(pcs) != 1 {
			return fail()
		}
		axis := 0
		fixed, e := torusPCurve(f, pcs[0], sid, 1)
		if e != nil {
			axis = 1
			fixed, e = torusPCurve(f, pcs[0], sid, 0)
		}
		if e != nil {
			return fail()
		}
		edges[i] = edge{start, end, axis, fixed, curve.Geometry}
	}
	boundary := make([]geom.Vec2, n)
	vertices := map[int64]bool{}
	for i, current := range edges {
		previous := edges[(i+n-1)%n]
		if current.axis == previous.axis {
			return fail()
		}
		if current.axis == 0 {
			boundary[i] = geom.Vec2{X: current.fixed, Y: previous.fixed}
		} else {
			boundary[i] = geom.Vec2{X: previous.fixed, Y: current.fixed}
		}
		start := current.start
		if !uses[i].sense {
			start = current.end
		}
		end := previous.end
		if !uses[(i+n-1)%n].sense {
			end = previous.start
		}
		if start != end || vertices[start] {
			return fail()
		}
		vertices[start] = true
	}
	exact, e := orthogonalBoundaries(support.Geometry, boundary)
	if e != nil {
		return fail()
	}
	o := geom.SurfacePatchOptions{MaxOperations: 200000, MaxCoefficientBits: 4096}
	for i, edge := range edges {
		a, b := boundary[i], boundary[(i+1)%n]
		forward := b.Y > a.Y
		if edge.axis == 1 {
			forward = b.X > a.X
		}
		if uses[i].sense != forward {
			return fail()
		}
		lo, hi := exact[i].Domain()
		model, e := geom.NewExactSplineCurve(edge.curve, o)
		if e != nil {
			return fail()
		}
		model, e = model.Restrict(lo, hi, o)
		if e != nil {
			return fail()
		}
		proof, e := geom.CompareExactCurves(model, exact[i], geom.ExactCurveComparisonOptions{MaxSpans: 64, MaxOperations: 200000, MaxCoefficientBits: 4096})
		if e != nil || !proof.Complete || !proof.Equal {
			return fail()
		}
		for j, vid := range []int64{edge.start, edge.end} {
			t := lo
			if j == 1 {
				t = hi
			}
			want, e := exact[i].ExactPoint(t)
			if e != nil {
				return fail()
			}
			got, e := analyticVertex(f, vid)
			if e != nil || got != want {
				return fail()
			}
		}
	}
	return OrthogonalSplineFace{Name: name, Geometry: support.Geometry, Boundary: boundary}, nil
}

// ReadOrthogonalSplineFaces proves nonrectangular iso-polygon boundaries on
// injective spline patches. It does not infer diagonal trims or shell topology.
func ReadOrthogonalSplineFaces(r io.Reader, l Limits) (OrthogonalSplineFaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return OrthogonalSplineFaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return OrthogonalSplineFaceModel{}, e
	}
	out := OrthogonalSplineFaceModel{}
	remaining := l.MaxEdgeUses
	m, e := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		n, e := orthogonalUseCount(f, id)
		if e != nil || n > remaining {
			return BoundedSplineFace{}, errors.New("step: orthogonal face use limit")
		}
		face, e := decodeOrthogonalSplineFace(f, id, maxPoints)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		remaining -= n
		out.Faces = append(out.Faces, face)
		d, _ := orthogonalPolygon(face.Boundary)
		return BoundedSplineFace{Name: face.Name, Geometry: face.Geometry, Domain: d}, nil
	}, 0)
	if e != nil {
		return OrthogonalSplineFaceModel{}, e
	}
	out.LengthUnit = m.LengthUnit
	return out, nil
}
