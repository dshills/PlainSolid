package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NestedSectionHistory records one material region and its shared outer and
// inner section edges. InnerContours index the certified contour traversal.
type NestedSectionHistory struct {
	OuterContour          int
	InnerContours         []int
	SplineFace, PlaneFace int
	OuterEdges            []int
	InnerEdges            [][]int
}

// SealNestedSplinePlaneSections seals all bounded regions of a projected
// spline whose boundary lies strictly on one side of the plane. The selected
// regions have the opposite sign, so holes and islands follow nesting parity.
// Cross-loop arc boxes must be strictly separated; this lets each contour be
// continuously replaced by its chord polygon for containment without crossing
// another contour. Unresolved or touching contour arrangements return errors.
func SealNestedSplinePlaneSections(ctx context.Context, surface *geom.BSplineSurface, height, tolerance float64, o intersect.SplinePlaneOptions, proof geom.SurfaceInjectivityOptions) (Solid, []NestedSectionHistory, error) {
	if ctx == nil {
		return Solid{}, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, nil, e
	}
	p, orientation, e := projectedSectionProof(surface, height, tolerance, o, proof)
	if e != nil {
		return Solid{}, nil, e
	}
	f, e := geom.FrameFromNormal(geom.Vec3{Z: height}, geom.Vec3{Z: 1})
	if e != nil {
		return Solid{}, nil, e
	}
	pl, e := geom.NewPlane(f.Origin(), f.ZAxis())
	if e != nil {
		return Solid{}, nil, e
	}
	sign, e := geom.ClassifyPlaneBoundarySigns(p, pl, geom.PlaneBoundarySignOptions{MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits})
	if e != nil {
		return Solid{}, nil, e
	}
	if !sign.Complete || sign.StrictSign == 0 {
		return Solid{}, nil, geom.ErrConvergence
	}
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = tolerance
	}
	loops, trace, e := intersect.TraceSplinePlaneContext(ctx, surface, f, o)
	if e != nil {
		return Solid{}, nil, e
	}
	if len(trace.Isolation.Tangencies) != 0 {
		return Solid{}, nil, geom.ErrDomain
	}
	return assembleNestedSplineSections(ctx, surface, nil, f, loops, p.Domain(), sign.StrictSign, orientation, tolerance)
}

func assembleNestedSplineSections(ctx context.Context, surface, second *geom.BSplineSurface, f geom.Frame, loops []intersect.CertifiedContour,
	d geom.ParameterRectangle, sign, orientation int, tolerance float64) (Solid, []NestedSectionHistory, error) {
	if len(loops) == 0 || len(loops) > 32 {
		return Solid{}, nil, geom.ErrDomain
	}
	type ring struct {
		points []geom.Vec2
		boxes  []geom.UVBox
	}
	rings := make([]ring, len(loops))
	segments := 0
	for i, c := range loops {
		if !c.Closed || len(c.Segments) < 3 || len(c.Segments) > brep.MaxHoledFaceVertices {
			return Solid{}, nil, geom.ErrDomain
		}
		segments += len(c.Segments)
		if segments > 1024 {
			return Solid{}, nil, parametric.ErrBudget
		}
		for _, s := range c.Segments {
			if e := ctx.Err(); e != nil {
				return Solid{}, nil, e
			}
			t := s.SplineTrim()
			a, e := t.EvalUV(0)
			if e != nil {
				return Solid{}, nil, e
			}
			if a.X <= d.UMin || a.X >= d.UMax || a.Y <= d.VMin || a.Y >= d.VMax {
				return Solid{}, nil, geom.ErrDomain
			}
			b, e := t.BoundsUV()
			if e != nil {
				return Solid{}, nil, e
			}
			rings[i].points = append(rings[i].points, a)
			rings[i].boxes = append(rings[i].boxes, b)
		}
		xyz := make([]geom.Vec3, len(rings[i].points))
		for j, q := range rings[i].points {
			xyz[j] = geom.Vec3{X: q.X, Y: q.Y}
		}
		if _, e := triangulateUV([][]geom.Vec3{xyz}); e != nil {
			return Solid{}, nil, fmt.Errorf("trimmed: contour chord polygon: %w", e)
		}
	}
	depth := make([]int, len(rings))
	inside := make([][]bool, len(rings))
	for i, a := range rings {
		inside[i] = make([]bool, len(rings))
		for j, b := range rings {
			if e := ctx.Err(); e != nil {
				return Solid{}, nil, e
			}
			if i == j {
				continue
			}
			if j > i {
				for _, x := range a.boxes {
					for _, y := range b.boxes {
						if x.Min.X <= y.Max.X && x.Max.X >= y.Min.X && x.Min.Y <= y.Max.Y && x.Max.Y >= y.Min.Y {
							return Solid{}, nil, fmt.Errorf("trimmed: unresolved separation of contour arcs")
						}
					}
				}
			}
			if winding(a.points[0], b.points) != 0 {
				inside[i][j] = true
				depth[i]++
			}
		}
	}
	parent := make([]int, len(rings))
	for i := range rings {
		parent[i] = -1
		for j := range rings {
			if !inside[i][j] {
				continue
			}
			if depth[j] >= depth[i] {
				return Solid{}, nil, fmt.Errorf("trimmed: inconsistent contour nesting")
			}
			if depth[j] == depth[i]-1 {
				if parent[i] >= 0 {
					return Solid{}, nil, fmt.Errorf("trimmed: ambiguous contour parent")
				}
				parent[i] = j
			}
		}
		if depth[i] > 0 && parent[i] < 0 {
			return Solid{}, nil, fmt.Errorf("trimmed: missing contour parent")
		}
	}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	var history []NestedSectionHistory
	ps, e := geom.NewPlaneSurface(f)
	if e != nil {
		return Solid{}, nil, e
	}
	below := sign > 0
	var secondSurface geom.Surface = ps
	if second != nil {
		secondSurface = second
	}
	for i := range loops {
		if depth[i]%2 != 0 {
			continue
		}
		h := NestedSectionHistory{OuterContour: i, SplineFace: len(faces), PlaneFace: len(faces) + 1}
		sf, pf := Face{Surface: surface, Reversed: below == (orientation > 0)}, Face{Surface: secondSurface, Reversed: !below}
		reverseSecond := orientation < 0
		if second != nil {
			pf.Reversed = !sf.Reversed
			reverseSecond = false
		}
		ids := []int{i}
		for j, p := range parent {
			if p == i {
				ids = append(ids, j)
				h.InnerContours = append(h.InnerContours, j)
			}
		}
		for k, id := range ids {
			if e := ctx.Err(); e != nil {
				return Solid{}, nil, e
			}
			c := loops[id]
			model, plane := make([]PCurve, len(c.Segments)), make([]PCurve, len(c.Segments))
			right := c.Segments[0].PlaneSignOnRight()
			if right == 0 {
				return Solid{}, nil, geom.ErrConvergence
			}
			// Right must face the nonmaterial side. That is the global
			// boundary sign for both outer rings and hole rings.
			reverse := right != sign
			n := len(c.Segments)
			for j, s := range c.Segments {
				if reverse {
					model[n-1-j] = reversePCurve(s.SplineTrim())
					plane[n-1-j] = reversePCurve(s.PlaneTrim())
					if second != nil {
						plane[n-1-j] = model[n-1-j]
					}
				} else {
					model[j], plane[j] = s.SplineTrim(), s.PlaneTrim()
					if second != nil {
						plane[j] = model[j]
					}
				}
			}
			mv, me := len(vertices), len(edges)
			ml, pp := make([]Use, n), make([]Use, n)
			ee := make([]int, n)
			for j := range n {
				a, e := model[j].EvalUV(0)
				if e != nil {
					return Solid{}, nil, e
				}
				b, e := model[j].EvalUV(1)
				if e != nil {
					return Solid{}, nil, e
				}
				m := surfaceTrimCurve{surface, a, b, model[j]}
				q, e := m.Eval(0)
				if e != nil {
					return Solid{}, nil, e
				}
				vertices = append(vertices, q)
				edges = append(edges, parametric.Edge{Start: mv + j, End: mv + (j+1)%n, Curve: m, Range: parametric.Interval{Max: 1}})
				ee[j] = me + j
				ml[j] = Use{Edge: me + j, UVStart: a, UVEnd: b, Curve: model[j]}
				u, e := plane[j].EvalUV(0)
				if e != nil {
					return Solid{}, nil, e
				}
				v, e := plane[j].EvalUV(1)
				if e != nil {
					return Solid{}, nil, e
				}
				jp := j
				if reverseSecond {
					jp = n - 1 - j
					u, v = v, u
					plane[j] = reversePCurve(plane[j])
				}
				pp[jp] = Use{Edge: me + j, Reversed: reverseSecond, UVStart: u, UVEnd: v, Curve: plane[j]}
			}
			if k == 0 {
				sf.Outer, pf.Outer = ml, pp
				h.OuterEdges = ee
			} else {
				sf.Holes, pf.Holes = append(sf.Holes, ml), append(pf.Holes, pp)
				h.InnerEdges = append(h.InnerEdges, ee)
			}
		}
		faces = append(faces, sf, pf)
		history = append(history, h)
	}
	s, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, nil, e
	}
	return s, history, nil
}

func winding(p geom.Vec2, ring []geom.Vec2) int {
	n := 0
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		if a.Y <= p.Y && p.Y < b.Y {
			if s, _ := geom.Orient2D(a, b, p); s > 0 {
				n++
			}
		}
		if b.Y <= p.Y && p.Y < a.Y {
			if s, _ := geom.Orient2D(a, b, p); s < 0 {
				n--
			}
		}
	}
	return n
}
