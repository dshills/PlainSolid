package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedQuadraticFaceOptions bounds all input trims (default/max32), all
// returned composition pieces (default4096, max65536), and cumulative work.
// Positive control hull bounds must be exactly representable and globally
// projection-injective across the original seam. This sufficient gate may
// reject a valid curve whose control hull is too large.
type WrappedQuadraticFaceOptions struct {
	geom.SurfaceInjectivityOptions
	MaxEdges, MaxPieces int
	Reversed            bool
}

// WrappedQuadraticFace retains original exact UV quadratics and original
// wrapped support. Rational seam/knot events split proof curves, not the native
// authored edge. This is an embedded open face, not a closed shell or solid.
type WrappedQuadraticFace struct {
	native     SewFace
	loops      [][]ExactQuadraticUV
	proofs     [][]geom.WrappedCurveComposition
	operations int
}

func (r *WrappedQuadraticFace) NativeFace() SewFace {
	if r == nil {
		return SewFace{}
	}
	return SewFace{Vertices: append([]geom.Vec3(nil), r.native.Vertices...), Edges: append([]parametric.Edge(nil), r.native.Edges...), Face: cloneFace(r.native.Face)}
}
func (r *WrappedQuadraticFace) OriginalLoops() [][]ExactQuadraticUV {
	if r == nil {
		return nil
	}
	out := make([][]ExactQuadraticUV, len(r.loops))
	for i, l := range r.loops {
		out[i] = append([]ExactQuadraticUV(nil), l...)
	}
	return out
}
func (r *WrappedQuadraticFace) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}

// Composition returns original authored edge parameters and lifted UV curves.
// Its mutable report/numbers are copied; model and UV curve objects immutable.
func (r *WrappedQuadraticFace) Composition(loop, edge int) (geom.WrappedCurveComposition, error) {
	if r == nil || loop < 0 || loop >= len(r.proofs) || edge < 0 || edge >= len(r.proofs[loop]) {
		return geom.WrappedCurveComposition{}, geom.ErrInvalid
	}
	out := r.proofs[loop][edge]
	out.Pieces = append([]geom.WrappedCurvePiece(nil), out.Pieces...)
	for i := range out.Pieces {
		p := &out.Pieces[i]
		p.Start = new(big.Rat).Set(p.Start)
		p.End = new(big.Rat).Set(p.End)
		for axis, k := range p.PeriodShift {
			p.PeriodShift[axis] = new(big.Int).Set(k)
		}
	}
	return out, nil
}

// NewWrappedQuadraticFace proves original exact loop topology, global lifted
// support embedding, complete rational seam composition and continuous native
// edge agreement. Outer winding is positive, holes negative. All native model
// and UV endpoints are lossless; algebraic seam events remain unsupported and
// no proof subinterval is replaced by a rounded parameter.
func NewWrappedQuadraticFace(ctx context.Context, w *geom.WrappedSurfacePatch, outer []ExactQuadraticUV, holes [][]ExactQuadraticUV, o WrappedQuadraticFaceOptions) (*WrappedQuadraticFace, error) {
	if ctx == nil || w == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 32
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEdges < 3 || o.MaxEdges > 32 || o.MaxPieces < 1 || o.MaxPieces > 65536 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || len(holes) > 9 {
		return nil, geom.ErrInvalid
	}
	n := len(outer)
	if n < 3 || n > o.MaxEdges {
		return nil, geom.ErrInvalid
	}
	for _, h := range holes {
		if len(h) < 3 || len(h) > o.MaxEdges-n {
			return nil, geom.ErrInvalid
		}
		n += len(h)
	}
	r := &WrappedQuadraticFace{loops: make([][]ExactQuadraticUV, 1+len(holes)), proofs: make([][]geom.WrappedCurveComposition, 1+len(holes))}
	r.loops[0] = append([]ExactQuadraticUV(nil), outer...)
	for i, h := range holes {
		r.loops[i+1] = append([]ExactQuadraticUV(nil), h...)
	}
	bounds := geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}
	trims := make([]trimvalidate.Loop, len(r.loops))
	used := 0
	for li, l := range r.loops {
		for _, q := range l {
			h := q.HomogeneousControls()
			seg, e := trimvalidate.ExactQuadratic(h)
			if e != nil {
				return nil, e
			}
			trims[li] = append(trims[li], seg)
			for _, p := range h {
				var xy [2]float64
				for axis := 0; axis < 2; axis++ {
					value := new(big.Rat).Quo(p[axis], p[2])
					used++
					if used > o.MaxOperations || value.Num().BitLen() > o.MaxCoefficientBits || value.Denom().BitLen() > o.MaxCoefficientBits {
						return nil, geom.ErrConvergence
					}
					xy[axis], e = geom.RepresentedExactScalar(value)
					if e != nil {
						return nil, e
					}
				}
				bounds.UMin = math.Min(bounds.UMin, xy[0])
				bounds.UMax = math.Max(bounds.UMax, xy[0])
				bounds.VMin = math.Min(bounds.VMin, xy[1])
				bounds.VMax = math.Max(bounds.VMax, xy[1])
			}
		}
	}
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	cert, report, e := trimvalidate.CertifyLoops(ctx, trims[0], trims[1:], trimvalidate.Options{MaxEdges: o.MaxEdges, MaxWork: o.MaxOperations - used, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if report.Status == trimvalidate.Invalid {
		return nil, geom.ErrDomain
	}
	if cert == nil || report.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	used += report.Work
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
	r.native.Face = Face{Surface: w, Reversed: o.Reversed}
	pieces := 0
	for li, l := range r.loops {
		base := len(r.native.Vertices)
		var uses []Use
		for i, q := range l {
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			uv, e := q.modelParameterCurve(ctx, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			used += uv.ExactOperations()
			if used >= o.MaxOperations || pieces >= o.MaxPieces {
				return nil, geom.ErrConvergence
			}
			p, e := w.ComposeParameterCurvePieces(ctx, uv, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits}, MaxPieces: o.MaxPieces - pieces})
			if e != nil {
				return nil, e
			}
			if !p.Complete || !p.Continuous {
				return nil, geom.ErrConvergence
			}
			used += p.ExactOperations
			pieces += len(p.Pieces)
			r.proofs[li] = append(r.proofs[li], p)
			exact, _ := q.Endpoints()
			start, e := saddleRepresentedUV(exact[0])
			if e != nil {
				return nil, e
			}
			end, e := saddleRepresentedUV(exact[1])
			if e != nil {
				return nil, e
			}
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			jet, e := w.EvaluateRational(ctx, exact[0][0], exact[0][1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			used += jet.ExactOperations
			point, e := geom.RepresentedExactPoint(jet.Point)
			if e != nil {
				return nil, e
			}
			r.native.Vertices = append(r.native.Vertices, point)
			id := len(r.native.Edges)
			edge := parametric.Edge{Start: base + i, End: base + (i+1)%len(l), Curve: surfaceTrimCurve{surface: w, a: start, b: end, curve: q}, Range: parametric.Interval{Max: 1}}
			use := Use{Edge: id, UVStart: start, UVEnd: end, Curve: q}
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			same, work, e := compareWrappedQuadraticUse(ctx, edge, w, use, edgecheck.Options{MaxWork: o.MaxOperations - used, MaxBits: o.MaxCoefficientBits, MaxIntervals: o.MaxPieces})
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
			used += work
			r.native.Edges = append(r.native.Edges, edge)
			uses = append(uses, use)
		}
		if li == 0 {
			r.native.Face.Outer = uses
		} else {
			r.native.Face.Holes = append(r.native.Face.Holes, uses)
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.operations = used
	return r, nil
}
