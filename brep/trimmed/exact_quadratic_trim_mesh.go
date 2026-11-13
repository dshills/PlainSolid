package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// ExactQuadraticTrimMeshOptions bounds exact knot composition and chord proofs
// independently. MaxInputBits defaults to16384 (maximum65536); up to4096 fixed
// normalized cuts are accepted. Tessellation's usual segment/check/depth and
// exact sample budgets apply across the entire result.
type ExactQuadraticTrimMeshOptions struct {
	Tessellation QuadraticTrimTessellationOptions
	Composition  geom.SurfaceCurveCompositionOptions
	MaxInputBits int
}

// CertifiedExactQuadraticTrimPolyline preserves the native exact trim directly.
// The promoted polyline's legacy SplineUV Source is deliberately empty; this
// Source is authoritative and no rounded spline conversion has occurred.
// Composition retains exact knot intervals, model pieces and continuity proof.
// ExactUV/ExactPoints and normalized Parameters preserve nonbinary samples.
type CertifiedExactQuadraticTrimPolyline struct {
	CertifiedRationalCutTrimPolyline
	Source      ExactQuadraticUV
	Composition geom.SurfaceCurveCompositionPieces
}

// TessellateExactQuadraticTrimCertified consumes native nonbinary homogeneous
// trim controls on C0 piecewise biquadratic positive rational supports. Exact
// geometry composition supplies knot cuts and whole-cell hull proofs; fixed
// cuts are retained. Adaptive acceptance uses exact homogeneous Bernstein
// residual bounds, including model-point rounding, in both directions.
// Irrational knot cuts and exhausted limits remain explicit unresolved errors.
// This curve primitive makes no loop topology, fill or embedding claim.
func TessellateExactQuadraticTrimCertified(ctx context.Context, p *geom.SurfacePatch, trim ExactQuadraticUV, cuts []*big.Rat, o ExactQuadraticTrimMeshOptions) (CertifiedExactQuadraticTrimPolyline, error) {
	empty := CertifiedExactQuadraticTrimPolyline{}
	if ctx == nil || p == nil || len(cuts) > 4096 || !finite(o.Tessellation.Deflection) || o.Tessellation.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 16384
	}
	if o.MaxInputBits < 1 || o.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	opts := o.Tessellation
	if opts.MaxSegments == 0 {
		opts.MaxSegments = 4096
	}
	if opts.MaxChecks == 0 {
		opts.MaxChecks = 2*opts.MaxSegments - 1
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 32
	}
	if opts.MaxSegments < 1 || opts.MaxSegments > 65536 || opts.MaxChecks < 1 || opts.MaxChecks > 1000000 || opts.MaxDepth < 1 || opts.MaxDepth > 64 {
		return empty, geom.ErrInvalid
	}
	mandatory := make([]*big.Rat, 0, len(cuts)+2)
	for _, q := range cuts {
		if q == nil || q.Sign() < 0 || q.Cmp(big.NewRat(1, 1)) > 0 || q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrInvalid
		}
		mandatory = append(mandatory, new(big.Rat).Set(q))
	}
	h := trim.HomogeneousControls()
	if h[0][0] == nil {
		return empty, geom.ErrInvalid
	}
	controls := make([][4]*big.Rat, 3)
	for i, control := range h {
		for _, v := range control {
			if v.Num().BitLen() > o.MaxInputBits || v.Denom().BitLen() > o.MaxInputBits {
				return empty, geom.ErrConvergence
			}
		}
		controls[i] = [4]*big.Rat{control[0], control[1], new(big.Rat), control[2]}
	}
	uv, e := geom.NewExactHomogeneousBezier(ctx, controls, o.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	composed, e := p.ComposeParameterCurvePieces(ctx, uv, o.Composition)
	if e != nil {
		return empty, e
	}
	if !composed.Complete || !composed.Continuous || len(composed.Pieces) == 0 {
		return empty, geom.ErrConvergence
	}
	proof := CertifiedMultiSpanTrimSeam{}
	proof.ParameterMaps[0] = ExactTrimParameterMap{Offset: new(big.Rat), Scale: big.NewRat(1, 1), Degree: 2}
	us, vs := p.KnotBreaks()
	for _, piece := range composed.Pieces {
		i, j := sort.SearchFloat64s(us, piece.Cell.UMin), sort.SearchFloat64s(vs, piece.Cell.VMin)
		if i+1 >= len(us) || j+1 >= len(vs) || us[i] != piece.Cell.UMin || vs[j] != piece.Cell.VMin || us[i+1] != piece.Cell.UMax || vs[j+1] != piece.Cell.VMax {
			return empty, geom.ErrConvergence
		}
		item := MultiSpanTrimSeamPiece{Min: piece.Start, Max: piece.End}
		item.Cells[0] = [2]int{i, j}
		proof.Pieces = append(proof.Pieces, item)
		mandatory = append(mandatory, new(big.Rat).Set(piece.Start), new(big.Rat).Set(piece.End))
	}
	sort.Slice(mandatory, func(i, j int) bool { return mandatory[i].Cmp(mandatory[j]) < 0 })
	unique := mandatory[:0]
	for _, q := range mandatory {
		if q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrConvergence
		}
		if len(unique) == 0 || q.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, q)
		}
	}
	mandatory = unique
	if len(mandatory)-1 > opts.MaxSegments {
		return empty, geom.ErrConvergence
	}
	polyline, e := tessellateExactTrimPartition(ctx, p, SplineUV{}, h, proof, mandatory, opts, o.MaxInputBits)
	if e != nil {
		return empty, e
	}
	return CertifiedExactQuadraticTrimPolyline{CertifiedRationalCutTrimPolyline: polyline, Source: trim, Composition: composed}, nil
}
