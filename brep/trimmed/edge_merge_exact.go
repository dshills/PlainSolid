package trimmed

import (
	"context"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// EdgeMergeHistory maps both original edge fragments to their shared result,
// and maps the deleted intermediate vertex to -1.
type EdgeMergeHistory struct {
	Edge, RemovedVertex int
	Edges, Vertices     []int
}

// MergeSplitSplineEdgeExact reverses an exact isoparametric edge split
// while keeping both adjacent faces. The original ranges must meet in
// increasing order, lie on the same immutable evaluator, and form an
// exact straight UV interval on each incident face. The connecting vertex
// must have no other incident edges.
func (s Solid) MergeSplitSplineEdgeExact(ctx context.Context, first, second int, o edgecheck.Options) (Solid, EdgeMergeHistory, error) {
	var h EdgeMergeHistory
	if ctx == nil || first < 0 || second < 0 || first == second || first >= len(s.edges) || second >= len(s.edges) {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	a, b := s.edges[first], s.edges[second]
	if a.End != b.Start || a.Start == a.End || b.Start == b.End || a.Range.Max != b.Range.Min ||
		!sameMergeSplineEvaluator(a.Curve, b.Curve) {
		return Solid{}, h, geom.ErrConvergence
	}
	middle := a.End
	for i, edge := range s.edges {
		if i != first && i != second && (edge.Start == middle || edge.End == middle) {
			return Solid{}, h, geom.ErrConvergence
		}
	}
	t := exactRectEdgeFraction(a.Range.Max, a.Range.Min, b.Range.Max)
	uses := 0
	faces := s.Faces()
	mergeLoop := func(loop []Use) ([]Use, error) {
		start := -1
		for i, u := range loop {
			if u.Edge == first && !u.Reversed || u.Edge == second && u.Reversed {
				start = i
				break
			}
		}
		if start < 0 {
			for _, u := range loop {
				if u.Edge == first || u.Edge == second {
					return nil, geom.ErrConvergence
				}
			}
			return loop, nil
		}
		var out []Use
		for k := 0; k < len(loop); k++ {
			u := loop[(start+k)%len(loop)]
			if u.Edge != first && u.Edge != second {
				out = append(out, u)
				continue
			}
			v := loop[(start+k+1)%len(loop)]
			if k+1 >= len(loop) || u.Reversed != v.Reversed || u.Curve != nil || v.Curve != nil ||
				u.UVEnd != v.UVStart || (!u.Reversed && (u.Edge != first || v.Edge != second)) ||
				(u.Reversed && (u.Edge != second || v.Edge != first)) ||
				(u.UVStart.X != v.UVEnd.X && u.UVStart.Y != v.UVEnd.Y) {
				return nil, geom.ErrConvergence
			}
			fraction := t
			if u.Reversed {
				fraction = ratInset(1)
				fraction.Sub(fraction, t)
			}
			x, okX := exactRectEdgeMix(fraction, u.UVStart.X, v.UVEnd.X)
			y, okY := exactRectEdgeMix(fraction, u.UVStart.Y, v.UVEnd.Y)
			if !okX || !okY || u.UVEnd != (geom.Vec2{X: x, Y: y}) {
				return nil, geom.ErrConvergence
			}
			out = append(out, Use{Edge: first, Reversed: u.Reversed, UVStart: u.UVStart, UVEnd: v.UVEnd})
			uses++
			k++
		}
		return out, nil
	}
	for i, f := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, h, e
		}
		var e error
		f.Outer, e = mergeLoop(f.Outer)
		if e != nil {
			return Solid{}, h, e
		}
		for j, l := range f.Holes {
			f.Holes[j], e = mergeLoop(l)
			if e != nil {
				return Solid{}, h, e
			}
		}
		faces[i] = f
	}
	if uses != 2 {
		return Solid{}, h, geom.ErrConvergence
	}
	h = EdgeMergeHistory{RemovedVertex: middle, Edges: deletedEntityMap(len(s.edges)), Vertices: deletedEntityMap(len(s.vertices))}
	var vertices []geom.Vec3
	for i, p := range s.vertices {
		if i != middle {
			h.Vertices[i] = len(vertices)
			vertices = append(vertices, p)
		}
	}
	var edges []parametric.Edge
	for i, edge := range s.edges {
		if i == second {
			continue
		}
		h.Edges[i] = len(edges)
		if i == first {
			edge.End, edge.Range.Max = b.End, b.Range.Max
		}
		edge.Start, edge.End = h.Vertices[edge.Start], h.Vertices[edge.End]
		edges = append(edges, edge)
	}
	h.Edge, h.Edges[second] = h.Edges[first], h.Edges[first]
	for i, f := range faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for j := range loop {
				loop[j].Edge = h.Edges[loop[j].Edge]
			}
		}
		faces[i] = f
	}
	out, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, EdgeMergeHistory{}, e
	}
	p, e := out.ValidateEdgeAgreement(ctx, o)
	if e != nil || !p.Complete || out.Topology().EulerCharacteristic != s.Topology().EulerCharacteristic {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, EdgeMergeHistory{}, e
	}
	for _, use := range p.Uses {
		if use.Edge == h.Edge && (use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0) {
			return Solid{}, EdgeMergeHistory{}, geom.ErrConvergence
		}
	}
	return out, h, nil
}

func sameMergeSplineEvaluator(a, b geom.Curve) bool {
	switch x := a.(type) {
	case *geom.BSpline:
		y, ok := b.(*geom.BSpline)
		return ok && x != nil && x == y
	case surfaceTrimCurve:
		y, ok := b.(surfaceTrimCurve)
		if !ok || x.curve != nil || y.curve != nil || x.a != y.a || x.b != y.b {
			return false
		}
		ax, am := parametric.SurfaceMapping(x.surface)
		bx, bm := parametric.SurfaceMapping(y.surface)
		if am != bm {
			return false
		}
		switch a := ax.(type) {
		case *geom.BSplineSurface:
			z, ok := bx.(*geom.BSplineSurface)
			if !ok || a == nil || z == nil {
				return false
			}
			if a == z {
				return true
			}
			// Independent native-decoded copies may still carry the
			// same immutable spline recipe. Exact comparison of the
			// merged use is checked below, against both incident faces.
			return geom.SameSplineSurface(a, z)
		case *geom.WrappedSurfacePatch:
			z, ok := bx.(*geom.WrappedSurfacePatch)
			return ok && a != nil && (a == z || geom.SameWrappedSurfaceRecipe(a, z))
		}
	}
	return false
}
