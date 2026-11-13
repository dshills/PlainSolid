package step

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
)

// ReadCurvedOvalFace accepts exactly one explicit open face on a single
// biquadratic patch with up to 16 interior oval holes. Each loop has four model
// curves that must equal exact surface composition
// of their owner pcurves. The first source bound must be the outer loop; holes
// follow it. No sampled agreement or planar approximation is used.
func ReadCurvedOvalFace(r io.Reader, l Limits, o CurvedOvalOptions) (CurvedOvalFace, error) {
	return readCurvedOvalFace(r, l, o, false)
}
func readCurvedOvalFace(r io.Reader, l Limits, o CurvedOvalOptions, ellipse bool) (CurvedOvalFace, error) {
	var result CurvedOvalFace
	fail := func() (CurvedOvalFace, error) {
		return CurvedOvalFace{}, fmt.Errorf("step: unsupported or inconsistent curved oval face")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return result, e
	}
	o, e = o.defaults()
	if e != nil {
		return result, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return result, e
	}
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if id <= 0 || owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	points, shells, models := 0, 0, 0
	for _, en := range f.Entities {
		for _, rec := range en.Records {
			switch rec.Type {
			case "CARTESIAN_POINT":
				points++
			case "OPEN_SHELL":
				shells++
			case "SHELL_BASED_SURFACE_MODEL":
				models++
			case "CLOSED_SHELL", "MANIFOLD_SOLID_BREP", "BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL":
				return fail()
			}
		}
	}
	if points > l.MaxVertices || shells != 1 || models != 1 {
		return fail()
	}
	graph, e := readSplineFaceGraph(f, l, func(f *File, fid int64, maxPoints int) (BoundedSplineFace, error) {
		bad := func() (BoundedSplineFace, error) { return BoundedSplineFace{}, geom.ErrInvalid }
		if result.SourceFaceID != 0 {
			return bad()
		}
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return bad()
		}
		sense, e := boolean(face.Args[3])
		if e != nil || !sense {
			return bad()
		}
		name, e := textValue(face.Args[0])
		if e != nil {
			return bad()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return bad()
		}
		if e = exactSplineWeightNormalization(f, sid, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
			return BoundedSplineFace{}, e
		}
		surface, e := DecodeBSplineSurface(f, sid, maxPoints)
		if e != nil || surface.ClosedU == "T" || surface.ClosedV == "T" || surface.SelfIntersect == "T" {
			return bad()
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) > 17 || (ellipse && len(bounds) != 1) || 4*len(bounds) > l.MaxEdgeUses {
			return bad()
		}
		result = CurvedOvalFace{Name: name, Geometry: surface.Geometry, SourceFaceID: fid}
		loopOptions := o
		loopOptions.MaxOperations /= len(bounds)
		if loopOptions.MaxOperations < 10 {
			return bad()
		}
		for loopIndex, boundID := range bounds {
			if !claim(boundID) {
				return bad()
			}
			bound, e := simple(f, boundID, "FACE_OUTER_BOUND", 3)
			if e != nil {
				bound, e = simple(f, boundID, "FACE_BOUND", 3)
			}
			if e != nil {
				return bad()
			}
			if loopIndex > 0 && bound.Type == "FACE_OUTER_BOUND" {
				return bad()
			}
			sense, e = boolean(bound.Args[2])
			if e != nil || !sense {
				return bad()
			}
			lid, e := ref(bound.Args[1])
			if e != nil || !claim(lid) {
				return bad()
			}
			loop, e := simple(f, lid, "EDGE_LOOP", 2)
			if e != nil {
				return bad()
			}
			uses, e := listRefs(loop.Args[1], 4)
			if e != nil || len(uses) != 4 {
				return bad()
			}
			var uv [4]*geom.BSpline
			var curves [4]*geom.ExactSplineCurve
			var edges [4]int64
			var first, previous int64
			for i, useID := range uses {
				if !claim(useID) {
					return bad()
				}
				use, e := simple(f, useID, "ORIENTED_EDGE", 5)
				if e != nil || use.Args[1].Kind != Derived || use.Args[2].Kind != Derived {
					return bad()
				}
				sense, e = boolean(use.Args[4])
				if e != nil || !sense {
					return bad()
				}
				eid, e := ref(use.Args[3])
				if e != nil || !claim(eid) {
					return bad()
				}
				edges[i] = eid
				edge, e := simple(f, eid, "EDGE_CURVE", 5)
				if e != nil {
					return bad()
				}
				sense, e = boolean(edge.Args[4])
				if e != nil || !sense {
					return bad()
				}
				start, e := ref(edge.Args[1])
				end, ee := ref(edge.Args[2])
				if e != nil || ee != nil || start == end || !claim(start) {
					return bad()
				}
				if i == 0 {
					first = start
				} else if start != previous {
					return bad()
				}
				previous = end
				scid, e := ref(edge.Args[3])
				if e != nil || !claim(scid) {
					return bad()
				}
				sc, e := simple(f, scid, "SURFACE_CURVE", 4)
				if e != nil || sc.Args[3].Kind != Enumeration || (sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "CURVE_3D") {
					return bad()
				}
				mid, e := ref(sc.Args[1])
				if e != nil || !claim(mid) {
					return bad()
				}
				pcs, e := listRefs(sc.Args[2], 1)
				if e != nil || len(pcs) != 1 || !claim(pcs[0]) {
					return bad()
				}
				pc, e := simple(f, pcs[0], "PCURVE", 3)
				if e != nil {
					return bad()
				}
				owner, e := ref(pc.Args[1])
				if e != nil || owner != sid {
					return bad()
				}
				rid, e := ref(pc.Args[2])
				if e != nil || !claim(rid) {
					return bad()
				}
				rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
				if e != nil {
					return bad()
				}
				ctx, e := ref(rep.Args[2])
				if e != nil || validatePCurveContext(f, ctx) != nil {
					return bad()
				}
				ids, e := listRefs(rep.Args[1], 1)
				if e != nil || len(ids) != 1 || !claim(ids[0]) {
					return bad()
				}
				uv[i], _, e = decodeCurvedOvalBezier(f, ids[0], true, o)
				if e != nil {
					return BoundedSplineFace{}, e
				}
				_, curves[i], e = decodeCurvedOvalBezier(f, mid, false, o)
				if e != nil {
					return BoundedSplineFace{}, e
				}
				for j, vid := range []int64{start, end} {
					v, e := simple(f, vid, "VERTEX_POINT", 2)
					if e != nil {
						return bad()
					}
					pid, e := ref(v.Args[1])
					if e != nil {
						return bad()
					}
					point, e := coordinate(f, pid, "CARTESIAN_POINT", 3)
					if e != nil {
						return bad()
					}
					want, e := curves[i].ExactPoint(float64(j))
					if e != nil || point != want {
						return bad()
					}
				}
			}
			if previous != first {
				return bad()
			}
			lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
			for _, c := range uv {
				if c.Degree() != 2 || len(c.Poles()) != 3 {
					return bad()
				}
				for _, p := range c.Poles() {
					lo[0] = min(lo[0], p.X)
					lo[1] = min(lo[1], p.Y)
					hi[0] = max(hi[0], p.X)
					hi[1] = max(hi[1], p.Y)
				}
			}
			center, radii := [2]float64{}, [2]float64{}
			for i := 0; i < 2; i++ {
				a, b := new(big.Rat).SetFloat64(lo[i]), new(big.Rat).SetFloat64(hi[i])
				center[i], e = geom.RepresentedExactScalar(new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1)))
				if e != nil {
					return BoundedSplineFace{}, e
				}
				radii[i], e = geom.RepresentedExactScalar(new(big.Rat).Quo(new(big.Rat).Sub(b, a), big.NewRat(2, 1)))
				if e != nil {
					return BoundedSplineFace{}, e
				}
			}
			q := QuadraticOval{Center: geom.Vec2{X: center[0], Y: center[1]}, Radii: geom.Vec2{X: radii[0], Y: radii[1]}, QuarterWeight: uv[0].Weights()[1]}
			shear := 0.
			if ellipse {
				q, shear, e = curvedEllipseDescriptor(uv)
				if e != nil {
					return BoundedSplineFace{}, e
				}
			}
			candidate := CurvedOvalFace{Name: name, Geometry: surface.Geometry, Outer: q, SourceFaceID: fid}
			candidate.ellipseShear = shear
			want, derived, e := certifyCurvedOvalMode(candidate, loopOptions, ellipse)
			if e != nil {
				return BoundedSplineFace{}, e
			}
			if loopIndex > 0 {
				var reversed [4]*geom.BSpline
				var reversedModels [4]*geom.ExactSplineCurve
				for i := 0; i < 4; i++ {
					p, w := want[3-i].Poles(), want[3-i].Weights()
					p[0], p[2] = p[2], p[0]
					w[0], w[2] = w[2], w[0]
					reversed[i], e = geom.NewBSpline(2, p, []float64{0, 0, 0, 1, 1, 1}, w)
					if e != nil {
						return bad()
					}
					reversedModels[i], e = derived[3-i].Reparameterize(0, 1, true, loopOptions.patch())
					if e != nil {
						return BoundedSplineFace{}, e
					}
				}
				want, derived = reversed, reversedModels
			}
			offset := -1
			equal := func(a, b *geom.BSpline) bool {
				ap, bp := a.Poles(), b.Poles()
				aw, bw := a.Weights(), b.Weights()
				if len(ap) != len(bp) {
					return false
				}
				for i := range ap {
					if ap[i] != bp[i] || aw[i] != bw[i] {
						return false
					}
				}
				return true
			}
			for k := 0; k < 4; k++ {
				if equal(uv[0], want[k]) {
					offset = k
				}
			}
			if offset < 0 {
				return bad()
			}
			for i := 0; i < 4; i++ {
				k := (offset + i) % 4
				if !equal(uv[i], want[k]) {
					return bad()
				}
				same, e := geom.CompareExactCurvesContext(context.Background(), curves[i], derived[k], geom.ExactCurveComparisonOptions{MaxOperations: loopOptions.MaxOperations / 10, MaxCoefficientBits: o.MaxCoefficientBits})
				if e != nil {
					return BoundedSplineFace{}, e
				}
				if !same.Complete || !same.Equal {
					return bad()
				}
				candidate.ModelCurves[k] = curves[i]
				candidate.SourceEdges[k] = edges[i]
			}
			if loopIndex == 0 {
				result = candidate
			} else {
				result.Holes = append(result.Holes, q)
				result.HoleModelCurves = append(result.HoleModelCurves, candidate.ModelCurves)
				result.SourceHoleEdges = append(result.SourceHoleEdges, candidate.SourceEdges)
			}
		}
		if e = validateCurvedOvalHoles(result); e != nil {
			return BoundedSplineFace{}, e
		}
		return BoundedSplineFace{Name: name, Geometry: surface.Geometry}, nil
	}, 4)
	if e != nil {
		return CurvedOvalFace{}, e
	}
	if len(graph.Faces) != 1 {
		return fail()
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			switch rec.Type {
			case "EDGE_CURVE", "ORIENTED_EDGE", "VERTEX_POINT", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "SURFACE_CURVE", "PCURVE", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS":
				if !owned[id] {
					return fail()
				}
			}
		}
	}
	result.LengthUnit = graph.LengthUnit
	return result, nil
}
