package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type polynomialClipSpanState struct {
	maxSpans int
	knots    []PolynomialPlaneKnot
}

// PolynomialPlaneKnotOwnership maps each exact source junction to one native
// vertex and its two ordered generated section edges.
type PolynomialPlaneKnotOwnership = QuadraticPlaneKnotOwnership

type PiecewisePolynomialPlaneClip struct {
	SplineCornerPlaneClip
	Knots []PolynomialPlaneKnotOwnership
}

// CombinePiecewisePolynomialSplinePlaneClip retains original multispan supports
// and emits one exact polynomial edge per certified free-axis span. Whole source
// embedding, full-range C0 seams, cap Jordan topology and continuous model
// pairing must all be proved. Internal dependent knots and nonrepresentable
// junctions remain unsupported. MaxSpans bounds each face; the complete section
// is limited to64 pieces. Each face tries at most two axes. Limits otherwise
// apply per span proof as documented by PolynomialPlaneSpanOptions.
func CombinePiecewisePolynomialSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o PolynomialPlaneSpanOptions) (PiecewisePolynomialPlaneClip, error) {
	var zero PiecewisePolynomialPlaneClip
	if o.MaxSpans == 0 {
		o.MaxSpans = 32
	}
	if o.MaxSpans < 1 || o.MaxSpans > 128 {
		return zero, geom.ErrInvalid
	}
	state := &polynomialClipSpanState{maxSpans: o.MaxSpans}
	r, e := combinePolynomialSplinePlaneClip(ctx, stock, tool, active, op, o.Faces, state)
	if e != nil {
		return zero, e
	}
	result := PiecewisePolynomialPlaneClip{SplineCornerPlaneClip: r}
	section := map[int]bool{}
	for _, id := range r.SectionEdges {
		section[id] = true
	}
	for _, k := range state.knots {
		var p geom.Vec3
		for axis, d := range []*float64{&p.X, &p.Y, &p.Z} {
			*d, e = geom.RepresentedExactScalar(k.Point[axis])
			if e != nil {
				return zero, e
			}
		}
		vertex := -1
		for i, v := range r.Solid.vertices {
			if v == p {
				if vertex >= 0 {
					return zero, geom.ErrConvergence
				}
				vertex = i
			}
		}
		if vertex < 0 || len(r.StockFaces[k.Face]) != 1 {
			return zero, geom.ErrConvergence
		}
		owned := PolynomialPlaneKnotOwnership{QuadraticPlaneKnot: k, Vertex: vertex, Edges: [2]int{-1, -1}}
		for _, u := range r.Solid.faces[r.StockFaces[k.Face][0]].Outer {
			edge := r.Solid.edges[u.Edge]
			if !section[u.Edge] || (edge.Start != vertex && edge.End != vertex) {
				continue
			}
			a, b := u.UVStart.X, u.UVEnd.X
			if k.FreeAxis == 1 {
				a, b = u.UVStart.Y, u.UVEnd.Y
			}
			x, e := geom.RepresentedExactScalar(k.Knot)
			if e != nil {
				return zero, e
			}
			side := -1
			if max(a, b) == x && min(a, b) < x {
				side = 0
			}
			if min(a, b) == x && max(a, b) > x {
				side = 1
			}
			if side < 0 || owned.Edges[side] >= 0 {
				return zero, geom.ErrConvergence
			}
			owned.Edges[side] = u.Edge
		}
		if owned.Edges[0] < 0 || owned.Edges[1] < 0 {
			return zero, geom.ErrConvergence
		}
		result.Knots = append(result.Knots, owned)
	}
	return result, nil
}
