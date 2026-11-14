package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedLoopFaceOptions bounds total vertices (default/max64) and cumulative
// exact proof work (default8M). Outer winding is CCW and hole winding is CW;
// Reversed changes physical face orientation without rewriting either winding.
type WrappedLoopFaceOptions struct {
	geom.SurfaceInjectivityOptions
	MaxVertices int
	Reversed    bool
}

// WrappedLoopFace is an immutable embedded open face with original straight
// lifted UV loops. Loop0 is the counterclockwise outer; subsequent loops are
// clockwise, strictly contained, disjoint nonnested holes. Nonconvex simple
// loops are supported when the entire enclosing rectangle is proved embedded.
// No loop is reduced modulo a period, snapped, reordered or approximated.
// Conceptual side IDs are (loop,index); no preexisting B-rep IDs are claimed.
type WrappedLoopFace struct {
	native             SewFace
	loops              [][]geom.Vec2
	bounds             geom.ParameterRectangle
	pieces, operations int
}

func (r *WrappedLoopFace) NativeFace() SewFace {
	if r == nil {
		return SewFace{}
	}
	return SewFace{Vertices: append([]geom.Vec3(nil), r.native.Vertices...), Edges: append([]parametric.Edge(nil), r.native.Edges...), Face: cloneFace(r.native.Face)}
}
func (r *WrappedLoopFace) OriginalLoops() [][]geom.Vec2 {
	if r == nil {
		return nil
	}
	out := make([][]geom.Vec2, len(r.loops))
	for i, l := range r.loops {
		out[i] = append([]geom.Vec2(nil), l...)
	}
	return out
}
func (r *WrappedLoopFace) LiftedBounds() geom.ParameterRectangle {
	if r == nil {
		return geom.ParameterRectangle{}
	}
	return r.bounds
}
func (r *WrappedLoopFace) FundamentalPieces() int {
	if r == nil {
		return 0
	}
	return r.pieces
}
func (r *WrappedLoopFace) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}

// OriginalSide preserves exact original endpoints in the input winding. Native
// edge indices enumerate loop0 then holes in input order, each with range[0,1].
func (r *WrappedLoopFace) OriginalSide(loop, side int) ([2][2]*big.Rat, error) {
	if r == nil || loop < 0 || loop >= len(r.loops) || side < 0 || side >= len(r.loops[loop]) {
		return [2][2]*big.Rat{}, geom.ErrInvalid
	}
	l := r.loops[loop]
	a, b := l[side], l[(side+1)%len(l)]
	return [2][2]*big.Rat{{ratInset(a.X), ratInset(a.Y)}, {ratInset(b.X), ratInset(b.Y)}}, nil
}

// NewWrappedLoopFace reuses exact rational trim validation, then proves global
// lifted injectivity on the containing rectangle and continuous native edge
// agreement. MaxVertices in o applies to ALL loops combined (default/max64).
// Reversed flips the face orientation without altering original loop winding.
// Curved pcurves are outside this straight-loop API. Native model vertices must
// be losslessly representable; failures are atomic and publish no partial face.
func NewWrappedLoopFace(ctx context.Context, w *geom.WrappedSurfacePatch, outer []geom.Vec2, holes [][]geom.Vec2, o WrappedLoopFaceOptions) (*WrappedLoopFace, error) {
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
	if o.MaxVertices < 3 || o.MaxVertices > 64 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || len(holes) > 20 {
		return nil, geom.ErrInvalid
	}
	count := len(outer)
	if count < 3 || count > o.MaxVertices {
		return nil, geom.ErrInvalid
	}
	for _, h := range holes {
		if len(h) < 3 || len(h) > o.MaxVertices-count {
			return nil, geom.ErrInvalid
		}
		count += len(h)
	}
	r := &WrappedLoopFace{loops: make([][]geom.Vec2, 1+len(holes))}
	r.loops[0] = append([]geom.Vec2(nil), outer...)
	for i, h := range holes {
		r.loops[i+1] = append([]geom.Vec2(nil), h...)
	}
	trims := make([]trimvalidate.Loop, len(r.loops))
	bounds := geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}
	for li, l := range r.loops {
		for i, p := range l {
			if !uvFinite(p) {
				return nil, geom.ErrInvalid
			}
			bounds.UMin = math.Min(bounds.UMin, p.X)
			bounds.UMax = math.Max(bounds.UMax, p.X)
			bounds.VMin = math.Min(bounds.VMin, p.Y)
			bounds.VMax = math.Max(bounds.VMax, p.Y)
			trims[li] = append(trims[li], trimvalidate.Segment{Start: p, End: l[(i+1)%len(l)]})
		}
	}
	cert, report, e := trimvalidate.CertifyLoops(ctx, trims[0], trims[1:], trimvalidate.Options{MaxEdges: o.MaxVertices, MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if report.Status == trimvalidate.Invalid {
		return nil, geom.ErrDomain
	}
	if cert == nil || report.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	used := report.Work
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
	for li, l := range r.loops {
		base := len(r.native.Vertices)
		var uses []Use
		for i, p := range l {
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			jet, e := w.EvaluateRational(ctx, ratInset(p.X), ratInset(p.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			used += jet.ExactOperations
			point, e := geom.RepresentedExactPoint(jet.Point)
			if e != nil {
				return nil, e
			}
			r.native.Vertices = append(r.native.Vertices, point)
			j := (i + 1) % len(l)
			id := len(r.native.Edges)
			r.native.Edges = append(r.native.Edges, parametric.Edge{Start: base + i, End: base + j, Curve: surfaceTrimCurve{surface: w, a: p, b: l[j]}, Range: parametric.Interval{Max: 1}})
			uses = append(uses, Use{Edge: id, UVStart: p, UVEnd: l[j]})
		}
		if li == 0 {
			r.native.Face.Outer = uses
		} else {
			r.native.Face.Holes = append(r.native.Face.Holes, uses)
		}
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
	loops := append([][]Use{r.native.Face.Outer}, r.native.Face.Holes...)
	for _, l := range loops {
		for _, u := range l {
			if o.MaxOperations-used < quota {
				return nil, geom.ErrConvergence
			}
			model, ok := analyticModelLine(r.native.Edges[u.Edge])
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
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.operations = used
	return r, nil
}
