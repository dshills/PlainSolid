package trimmed

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineShellOverlapHistory reports all output fragments retained from each
// input face. Empty entries mean that the face was removed. The interface
// map applies to the closed overlap rectangle in each chart.
type SplineShellOverlapHistory struct {
	InterfaceA, InterfaceB int
	Map                    intersect.SplineChartMap
	FacesA, FacesB         [][]int
}

// CombineOverlappingInterfaceSplineShells joins two certified spline shells
// that occupy opposite sides of a common plane. Both interface faces must
// have affine charts with aligned axes (transposition/reversal is allowed).
// Their positive-area intersection is removed from both faces during union;
// splits on original edges are propagated to the other incident faces.
// Regularized intersection is empty and difference leaves A intact.
func CombineOverlappingInterfaceSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellJoinOptions) (Solid, SplineShellOverlapHistory, error) {
	h := SplineShellOverlapHistory{InterfaceA: -1, InterfaceB: -1}
	if ctx == nil || op > parametric.Difference || len(a.faces) == 0 || len(b.faces) == 0 {
		return Solid{}, h, geom.ErrInvalid
	}
	var ap, bp []splineEmbeddingFace
	pa, e := a.validateSplineRectangleEmbedding(ctx, o.Embedding, &ap)
	if e != nil {
		return Solid{}, h, e
	}
	pb, e := b.validateSplineRectangleEmbedding(ctx, o.Embedding, &bp)
	if e != nil {
		return Solid{}, h, e
	}
	charts := func(f []splineEmbeddingFace, p SplineRectangleEmbedding) ([]*affineInsetChart, error) {
		out := make([]*affineInsetChart, len(f))
		for i, x := range f {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			c, e := splineAffineChart(ctx, x.patch, p.Faces[i].ProjectionAxes, o.Coincidence)
			if e != nil {
				return nil, e
			}
			out[i] = c
		}
		return out, nil
	}
	ac, e := charts(ap, pa)
	if e != nil {
		return Solid{}, h, e
	}
	bc, e := charts(bp, pb)
	if e != nil {
		return Solid{}, h, e
	}
	for i, x := range ac {
		if x == nil {
			continue
		}
		for j, y := range bc {
			if e := ctx.Err(); e != nil {
				return Solid{}, h, e
			}
			if y == nil {
				continue
			}
			var from [4]geom.Vec3
			for k, id := range bp[j].vertex {
				from[k] = b.vertices[id]
			}
			d, ok := x.rectangularProjection(from)
			if !ok {
				continue
			}
			da := intersectRectangles(x.domain, d)
			if !da.Valid() {
				continue
			}
			// Project the overlap vertices into B using the exact A chart.
			var points [4]geom.Vec3
			for k, uv := range insetCorners(da) {
				points[k], ok = x.point(uv)
				if !ok {
					break
				}
			}
			if !ok {
				continue
			}
			db, ok := y.rectangularProjection(points)
			if !ok || intersectRectangles(y.domain, db) != db {
				continue
			}
			xp, e := ap[i].patch.Trim(da)
			if e != nil {
				return Solid{}, h, e
			}
			yp, e := bp[j].patch.Trim(db)
			if e != nil {
				return Solid{}, h, e
			}
			q, e := intersect.FindExactSplineCoincidence(ctx, xp, yp, o.Coincidence)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return Solid{}, h, e
			}
			if !q.Found {
				continue
			}
			as, e := proveSplineShellOnInterfaceSide(ctx, ap, i, ap[i].plane, o.Embedding.Plane)
			if e != nil {
				return Solid{}, h, e
			}
			bs, e := proveSplineShellOnInterfaceSide(ctx, bp, j, ap[i].plane, o.Embedding.Plane)
			if e != nil {
				return Solid{}, h, e
			}
			if as == 0 || bs != -as {
				continue
			}
			h.InterfaceA, h.InterfaceB, h.Map = i, j, q.Map
			return joinOverlapSplineShells(ctx, a, b, op, h, x, y, o.Sew)
		}
	}
	return Solid{}, h, fmt.Errorf("%w: no exact planar rectangle overlap between the spline shells", geom.ErrConvergence)
}

func intersectRectangles(a, b geom.ParameterRectangle) geom.ParameterRectangle {
	return geom.ParameterRectangle{UMin: max(a.UMin, b.UMin), UMax: min(a.UMax, b.UMax), VMin: max(a.VMin, b.VMin), VMax: min(a.VMax, b.VMax)}
}

func joinOverlapSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, h SplineShellOverlapHistory,
	ac, bc *affineInsetChart, o SewOptions) (Solid, SplineShellOverlapHistory, error) {
	h.FacesA, h.FacesB = make([][]int, len(a.faces)), make([][]int, len(b.faces))
	if op == parametric.Intersection {
		return Solid{}, h, nil
	}
	if op == parametric.Difference {
		for i := range h.FacesA {
			h.FacesA[i] = []int{i}
		}
		return a, h, nil
	}
	var all []SewFace
	for _, x := range []struct {
		s       Solid
		chart   *affineInsetChart
		overlap geom.ParameterRectangle
		face    int
		history [][]int
	}{{a, ac, h.Map.DomainA, h.InterfaceA, h.FacesA}, {b, bc, h.Map.DomainB, h.InterfaceB, h.FacesB}} {
		parts, history, e := x.s.splineInterfaceFragments(ctx, x.face, x.chart, x.overlap)
		if e != nil {
			return Solid{}, SplineShellOverlapHistory{}, e
		}
		for i, row := range history {
			for _, id := range row {
				x.history[i] = append(x.history[i], id+len(all))
			}
		}
		all = append(all, parts...)
	}
	if o.Tolerance == 0 {
		o.Tolerance = max(a.tolerance, b.tolerance)
	}
	s, _, e := Sew(ctx, all, o)
	if e != nil {
		return Solid{}, SplineShellOverlapHistory{}, e
	}
	return s, h, nil
}

func interfaceGrid(a, cut geom.ParameterRectangle) ([]float64, []float64) {
	return slices.Compact([]float64{a.UMin, cut.UMin, cut.UMax, a.UMax}),
		slices.Compact([]float64{a.VMin, cut.VMin, cut.VMax, a.VMax})
}

type splineInterfaceEdgeCut struct {
	fraction, parameter float64
	point               geom.Vec3
}

func (s Solid) splineInterfaceFragments(ctx context.Context, face int, chart *affineInsetChart, overlap geom.ParameterRectangle) ([]SewFace, [][]int, error) {
	u, v := interfaceGrid(chart.domain, overlap)
	points := map[geom.Vec2]geom.Vec3{}
	for _, x := range u {
		for _, y := range v {
			q := geom.Vec2{X: x, Y: y}
			p, ok := chart.point(q)
			if !ok {
				return nil, nil, geom.ErrUnrepresentable
			}
			points[q] = p
		}
	}
	cuts := make(map[int][]splineInterfaceEdgeCut)
	for _, use := range s.faces[face].Outer {
		for q, point := range points {
			start, end, at := use.UVStart.X, use.UVEnd.X, q.X
			if start == end {
				if q.X != start {
					continue
				}
				start, end, at = use.UVStart.Y, use.UVEnd.Y, q.Y
			} else if q.Y != use.UVStart.Y {
				continue
			}
			if at <= min(start, end) || at >= max(start, end) {
				continue
			}
			if use.Reversed {
				start, end = end, start
			}
			t := exactRectEdgeFraction(at, start, end)
			edge := s.edges[use.Edge]
			q, qe := exactRectEdgeMix(t, edge.Range.Min, edge.Range.Max)
			f, fe := t.Float64()
			if !qe || !fe || f <= 0 || f >= 1 || q <= edge.Range.Min || q >= edge.Range.Max {
				return nil, nil, geom.ErrUnrepresentable
			}
			cuts[use.Edge] = append(cuts[use.Edge], splineInterfaceEdgeCut{f, q, point})
		}
		slices.SortFunc(cuts[use.Edge], func(a, b splineInterfaceEdgeCut) int {
			if a.fraction < b.fraction {
				return -1
			}
			if a.fraction > b.fraction {
				return 1
			}
			return 0
		})
	}
	var parts []SewFace
	h := make([][]int, len(s.faces))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		if i != face {
			p, e := s.splineSewFaceWithSplits(f, cuts)
			if e != nil {
				return nil, nil, e
			}
			h[i] = []int{len(parts)}
			parts = append(parts, p)
			continue
		}
		for x := 0; x < len(u)-1; x++ {
			for y := 0; y < len(v)-1; y++ {
				d := geom.ParameterRectangle{UMin: u[x], UMax: u[x+1], VMin: v[y], VMax: v[y+1]}
				if intersectRectangles(d, overlap) == d {
					continue
				}
				h[i] = append(h[i], len(parts))
				uv := insetCorners(d)
				p := SewFace{Face: Face{Surface: f.Surface, Reversed: f.Reversed}}
				for k, x := range uv {
					j := (k + 1) % 4
					p.Vertices = append(p.Vertices, points[x])
					p.Edges = append(p.Edges, parametric.Edge{Start: k, End: j,
						Curve: surfaceTrimCurve{f.Surface, x, uv[j], nil}, Range: parametric.Interval{Max: 1}})
					p.Face.Outer = append(p.Face.Outer, Use{Edge: k, UVStart: x, UVEnd: uv[j]})
				}
				parts = append(parts, p)
			}
		}
	}
	return parts, h, nil
}

func (s Solid) splineSewFaceWithSplits(f Face, cuts map[int][]splineInterfaceEdgeCut) (SewFace, error) {
	p := SewFace{Face: Face{Surface: f.Surface, Reversed: f.Reversed}}
	vertices := map[geom.Vec3]int{}
	vertex := func(x geom.Vec3) int {
		if i, ok := vertices[x]; ok {
			return i
		}
		i := len(p.Vertices)
		vertices[x] = i
		p.Vertices = append(p.Vertices, x)
		return i
	}
	for _, use := range f.Outer {
		edge := s.edges[use.Edge]
		points := append([]splineInterfaceEdgeCut{{0, edge.Range.Min, s.vertices[edge.Start]}}, cuts[use.Edge]...)
		points = append(points, splineInterfaceEdgeCut{1, edge.Range.Max, s.vertices[edge.End]})
		uv := make([]geom.Vec2, len(points))
		for k, cut := range points {
			a, b := use.UVStart, use.UVEnd
			if use.Reversed {
				a, b = b, a
			}
			x, xe := exactFloatMix(a.X, b.X, cut.fraction)
			y, ye := exactFloatMix(a.Y, b.Y, cut.fraction)
			if !xe || !ye {
				return SewFace{}, geom.ErrUnrepresentable
			}
			uv[k] = geom.Vec2{X: x, Y: y}
		}
		for step := 0; step < len(points)-1; step++ {
			j := step
			if use.Reversed {
				j = len(points) - 2 - step
			}
			q, r := points[j], points[j+1]
			x, y := uv[j], uv[j+1]
			if use.Reversed {
				x, y = y, x
			}
			i := len(p.Edges)
			p.Edges = append(p.Edges, parametric.Edge{Start: vertex(q.point), End: vertex(r.point), Curve: edge.Curve,
				Range: parametric.Interval{Min: q.parameter, Max: r.parameter}})
			p.Face.Outer = append(p.Face.Outer, Use{Edge: i, Reversed: use.Reversed, UVStart: x, UVEnd: y})
		}
	}
	return p, nil
}
