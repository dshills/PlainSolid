package step

import (
	"errors"
	"math"
	"plainsolid/geom"
)

// BoundedSplineFace retains an exact rectangular trim on its original support.
type BoundedSplineFace struct {
	Name     string
	Geometry *geom.BSplineSurface
	Domain   geom.ParameterRectangle
}
type BoundedSplineFaceModel struct {
	Faces      []BoundedSplineFace
	LengthUnit float64
}

func boundedSplineBoundaries(s *geom.BSplineSurface, d geom.ParameterRectangle) ([4]*geom.ExactSplineCurve, error) {
	var curves [4]*geom.ExactSplineCurve
	o := geom.SurfacePatchOptions{MaxOperations: 200000, MaxCoefficientBits: 4096}
	patch, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		return curves, e
	}
	patch, e = patch.Trim(d)
	if e != nil {
		return curves, e
	}
	if e := certifySplineFacePatch(patch); e != nil {
		return curves, e
	}
	for j, fixed := range []float64{d.UMin, d.VMin, d.UMax, d.VMax} {
		axis := j % 2
		c, e := patch.IsoCurve(axis, fixed, o)
		if e != nil {
			return curves, e
		}
		curves[j] = c
	}
	return curves, nil
}
func decodeBoundedSplineFace(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
	fail := func() (BoundedSplineFace, error) {
		return BoundedSplineFace{}, errors.New("step: inconsistent bounded spline face graph")
	}
	face, uses, e := analyticFaceLoop(f, id, 4)
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
		return BoundedSplineFace{}, e
	}
	support, e := DecodeBSplineSurface(f, sid, maxPoints)
	if e != nil {
		return fail()
	}
	type boundary struct {
		start, end int64
		axis       int
		fixed      float64
		curve      *geom.BSpline
	}
	edges := map[int64]boundary{}
	us, vs := []float64{}, []float64{}
	for _, use := range uses {
		start, end, g, e := analyticEdge(f, use.id)
		if e != nil || start == end {
			return fail()
		}
		curve, e := simple(f, g, "SURFACE_CURVE", 4)
		if e != nil || curve.Args[3].Kind != Enumeration || curve.Args[3].Text != "PCURVE_S1" {
			return fail()
		}
		cid, e := ref(curve.Args[1])
		if e != nil {
			return fail()
		}
		if e := exactSplineWeightNormalization(f, cid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
			return BoundedSplineFace{}, e
		}
		c, e := DecodeBSplineCurve(f, cid, maxPoints)
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(curve.Args[2], 1)
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
		if axis == 0 {
			us = append(us, fixed)
		} else {
			vs = append(vs, fixed)
		}
		edges[use.id] = boundary{start, end, axis, fixed, c.Geometry}
	}
	if len(us) != 2 || len(vs) != 2 || us[0] == us[1] || vs[0] == vs[1] {
		return fail()
	}
	domain := geom.ParameterRectangle{UMin: math.Min(us[0], us[1]), UMax: math.Max(us[0], us[1]), VMin: math.Min(vs[0], vs[1]), VMax: math.Max(vs[0], vs[1])}
	expected, e := boundedSplineBoundaries(support.Geometry, domain)
	if e != nil {
		return fail()
	}
	sideByEdge := map[int64]int{}
	vertexIDs := map[int]int64{}
	cornerPoints := map[int]geom.Vec3{}
	seenSides := map[int]bool{}
	o := geom.SurfacePatchOptions{MaxOperations: 200000, MaxCoefficientBits: 4096}
	for id, b := range edges {
		side := b.axis
		if b.axis == 0 && b.fixed == domain.UMax {
			side = 2
		}
		if b.axis == 1 && b.fixed == domain.VMax {
			side = 3
		}
		if seenSides[side] {
			return fail()
		}
		seenSides[side] = true
		sideByEdge[id] = side
		a, z := expected[side].Domain()
		model, e := geom.NewExactSplineCurve(b.curve, o)
		if e != nil {
			return fail()
		}
		model, e = model.Restrict(a, z, o)
		if e != nil {
			return fail()
		}
		proof, e := geom.CompareExactCurves(model, expected[side], geom.ExactCurveComparisonOptions{MaxSpans: 64, MaxOperations: 200000, MaxCoefficientBits: 4096})
		if e != nil || !proof.Complete || !proof.Equal {
			return fail()
		}
		for k, v := range []int64{b.start, b.end} {
			parameter := a
			if k == 1 {
				parameter = z
			}
			p, e := expected[side].ExactPoint(parameter)
			if e != nil {
				return fail()
			}
			vertex, e := analyticVertex(f, v)
			if e != nil || vertex != p {
				return fail()
			}
			corner := splineFaceEnds[side][k]
			if old, ok := vertexIDs[corner]; ok && old != v {
				return fail()
			}
			vertexIDs[corner] = v
			cornerPoints[corner] = p
		}
	}
	for i, u := range uses {
		side := sideByEdge[u.id]
		next := sideByEdge[uses[(i+1)%4].id]
		if next != (side+1)%4 || u.sense != (side == 1 || side == 2) {
			return fail()
		}
	}
	if len(vertexIDs) != 4 {
		return fail()
	}
	for i, p := range cornerPoints {
		for j, q := range cornerPoints {
			if i != j && p == q {
				return fail()
			}
		}
	}
	return BoundedSplineFace{name, support.Geometry, domain}, nil
}
