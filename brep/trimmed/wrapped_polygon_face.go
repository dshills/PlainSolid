//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedPolygonFaceOptions shares one exact work quota across convexity,
// global lifted embedding, model vertices and continuous edge agreement.
// MaxVertices defaults to64 and cannot exceed64. Reversed changes the surface
// orientation only; input UV must remain strictly convex counterclockwise.
type WrappedPolygonFaceOptions struct {
	geom.SurfaceInjectivityOptions
	MaxVertices int
	Reversed    bool
}

// WrappedPolygonFace is one embedded open face on its immutable original
// wrapped support. Straight UV edges may cross a fundamental seam and become
// curved model edges. This is not a planar model polygon or a closed solid.
// Original vertex order supplies conceptual side IDs; there are no input
// B-rep edge IDs. Curved pcurves, holes and nonconvex polygons are unsupported.
type WrappedPolygonFace struct {
	native             SewFace
	uv                 []geom.Vec2
	bounds             geom.ParameterRectangle
	pieces, operations int
}

func (r *WrappedPolygonFace) NativeFace() SewFace {
	if r == nil {
		return SewFace{}
	}
	return SewFace{Vertices: append([]geom.Vec3(nil), r.native.Vertices...), Edges: append([]parametric.Edge(nil), r.native.Edges...), Face: cloneFace(r.native.Face)}
}
func (r *WrappedPolygonFace) OriginalUV() []geom.Vec2 {
	if r == nil {
		return nil
	}
	return append([]geom.Vec2(nil), r.uv...)
}
func (r *WrappedPolygonFace) LiftedBounds() geom.ParameterRectangle {
	if r == nil {
		return geom.ParameterRectangle{}
	}
	return r.bounds
}
func (r *WrappedPolygonFace) FundamentalPieces() int {
	if r == nil {
		return 0
	}
	return r.pieces
}
func (r *WrappedPolygonFace) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}

// OriginalSide returns the exact original lifted endpoint pair of a conceptual
// side. Output model edge i has this same direction and normalized range[0,1].
func (r *WrappedPolygonFace) OriginalSide(i int) ([2][2]*big.Rat, error) {
	if r == nil || i < 0 || i >= len(r.uv) {
		return [2][2]*big.Rat{}, geom.ErrInvalid
	}
	a, b := r.uv[i], r.uv[(i+1)%len(r.uv)]
	return [2][2]*big.Rat{{ratInset(a.X), ratInset(a.Y)}, {ratInset(b.X), ratInset(b.Y)}}, nil
}

// NewWrappedPolygonFace accepts a strictly convex represented UV polygon and
// proves injectivity on its entire containing lifted rectangle using one
// global projected monotonicity certificate across all seam restrictions.
// Thus boundary and interior embedding are proved together, not inferred from
// local edge samples. Model vertices must be exactly representable. Failure
// returns no partial face; caller slices are never retained.
func NewWrappedPolygonFace(ctx context.Context, w *geom.WrappedSurfacePatch, uv []geom.Vec2, o WrappedPolygonFaceOptions) (*WrappedPolygonFace, error) {
	if ctx == nil || w == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxVertices < 3 || o.MaxVertices > 64 || len(uv) < 3 || len(uv) > o.MaxVertices || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	used := 0
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	r := &WrappedPolygonFace{uv: append([]geom.Vec2(nil), uv...)}
	points := make([][2]*big.Rat, len(uv))
	bounds := geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}
	for i, p := range uv {
		if !uvFinite(p) {
			return nil, geom.ErrInvalid
		}
		points[i] = [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}
		for _, q := range points[i] {
			if e := check(q); e != nil {
				return nil, e
			}
		}
		bounds.UMin = math.Min(bounds.UMin, p.X)
		bounds.UMax = math.Max(bounds.UMax, p.X)
		bounds.VMin = math.Min(bounds.VMin, p.Y)
		bounds.VMax = math.Max(bounds.VMax, p.Y)
	}
	// Every other vertex must lie strictly left of each directed edge. This
	// proves the supplied cyclic order is the simple strict convex hull.
	for i, a := range points {
		j := (i + 1) % len(points)
		b := points[j]
		dx, dy := new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(b[1], a[1])
		for k, p := range points {
			if k == i || k == j {
				continue
			}
			cross := new(big.Rat).Sub(new(big.Rat).Mul(dx, new(big.Rat).Sub(p[1], a[1])), new(big.Rat).Mul(dy, new(big.Rat).Sub(p[0], a[0])))
			if e := check(cross); e != nil {
				return nil, e
			}
			if cross.Sign() <= 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	io := o.SurfaceInjectivityOptions
	io.MaxOperations = o.MaxOperations - used
	embedding, e := w.CertifyLiftedRectangleInjectivity(ctx, bounds, io)
	if e != nil {
		return nil, e
	}
	if !embedding.ProvenInjective {
		return nil, geom.ErrConvergence
	}
	used += embedding.ExactOperations
	r.bounds = bounds
	r.pieces = embedding.FundamentalPieces
	r.native.Face = Face{Surface: w, Reversed: o.Reversed}
	for i, p := range points {
		if used >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		jet, e := w.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
		used += jet.ExactOperations
		point, e := geom.RepresentedExactPoint(jet.Point)
		if e != nil {
			return nil, e
		}
		r.native.Vertices = append(r.native.Vertices, point)
		j := (i + 1) % len(points)
		r.native.Edges = append(r.native.Edges, parametric.Edge{Start: i, End: j, Curve: surfaceTrimCurve{surface: w, a: uv[i], b: uv[j]}, Range: parametric.Interval{Max: 1}})
		r.native.Face.Outer = append(r.native.Face.Outer, Use{Edge: i, UVStart: uv[i], UVEnd: uv[j]})
	}
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	relation, proof, e := geom.CompareWrappedSurfaceCharts(ctx, w, w, geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: o.MaxCells})
	if e != nil {
		return nil, e
	}
	if relation == nil || !proof.Complete || !proof.Equal {
		return nil, geom.ErrConvergence
	}
	used += proof.ExactOperations
	const quota = 64
	for i, u := range r.native.Face.Outer {
		if o.MaxOperations-used < quota {
			return nil, geom.ErrConvergence
		}
		model, ok := analyticModelLine(r.native.Edges[i])
		if !ok {
			return nil, geom.ErrConvergence
		}
		use, ok := analyticUseLine(w, u)
		if !ok {
			return nil, geom.ErrConvergence
		}
		same, e := relation.SameParameterLines(ctx, model.uv, use.uv, geom.SurfacePatchOptions{MaxOperations: quota, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
		if !same {
			return nil, geom.ErrConvergence
		}
		used += quota
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.operations = used
	return r, nil
}
