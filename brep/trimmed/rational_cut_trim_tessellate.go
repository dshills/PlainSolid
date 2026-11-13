package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// RationalCutTrimOptions bounds adaptive chord checks and exact knot discovery.
// MaxInputBits defaults to16384 (maximum65536), including supplied cut integers.
// At most4096 supplied cuts are accepted. Partition bounds all knot tests and
// continuous cell identity checks; Tessellation bounds all adaptive subpieces.
type RationalCutTrimOptions struct {
	Tessellation QuadraticTrimTessellationOptions
	Partition    MultiSpanTrimSeamOptions
	MaxInputBits int
}

// CertifiedRationalCutTrimPolyline retains exact normalized and original source
// parameters, exact UV/model samples, and nearest binary64 display points.
// FixedCuts contains all mandatory user/knot cuts, sorted and deduplicated.
// Cells identifies the original support knot rectangle of each chord.
type CertifiedRationalCutTrimPolyline struct {
	Source                                      SplineUV // Original immutable curve and directed parameter range.
	Points                                      []geom.Vec3
	ExactPoints                                 [][3]*big.Rat
	ExactUV                                     [][2]*big.Rat
	Parameters, NormalizedParameters, FixedCuts []*big.Rat
	SquaredErrors                               []*big.Rat
	Cells                                       [][2]int
	Checks                                      int
	ParameterMap                                ExactTrimParameterMap
}

// TessellateRationalCutSplineTrimCertified accepts one positive rational linear
// or quadratic Bézier UV trim on a C0 piecewise biquadratic rational support.
// User cuts use normalized directed traversal, not absolute curve parameters.
// Every exact rational knot crossing is inserted automatically; irrational
// crossings explicitly remain unresolved. Chords never straddle knot cells.
// Restricted homogeneous control points are kept rational, never reconstructed
// as rounded spline poles or binary64 source ranges. Every chord has a rigorous
// two-sided common-parameter bound including display rounding. This primitive
// does not certify loop topology or a filled face. All failures are atomic.
func TessellateRationalCutSplineTrimCertified(ctx context.Context, p *geom.SurfacePatch, trim SplineUV, cuts []*big.Rat, o RationalCutTrimOptions) (CertifiedRationalCutTrimPolyline, error) {
	empty := CertifiedRationalCutTrimPolyline{}
	if ctx == nil || p == nil || !finite(o.Tessellation.Deflection) || o.Tessellation.Deflection <= 0 || len(cuts) > 4096 {
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
	mandatory := []*big.Rat{}
	for _, q := range cuts {
		if q == nil || q.Sign() < 0 || q.Cmp(big.NewRat(1, 1)) > 0 || q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrInvalid
		}
		mandatory = append(mandatory, new(big.Rat).Set(q))
	}
	proof, e := CertifyMultiSpanTrimSeam(ctx, p, p, trim, trim, o.Partition)
	if e != nil {
		return empty, e
	}
	for _, piece := range proof.Pieces {
		mandatory = append(mandatory, new(big.Rat).Set(piece.Min), new(big.Rat).Set(piece.Max))
	}
	sort.Slice(mandatory, func(i, j int) bool { return mandatory[i].Cmp(mandatory[j]) < 0 })
	unique := mandatory[:0]
	for _, q := range mandatory {
		if len(unique) == 0 || q.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, q)
		}
	}
	mandatory = unique
	for _, q := range mandatory {
		if q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrConvergence
		}
	}
	if len(mandatory)-1 > opts.MaxSegments {
		return empty, geom.ErrConvergence
	}
	h, e := quadraticTrimSourceControls(trim)
	if e != nil {
		return empty, e
	}
	return tessellateExactTrimPartition(ctx, p, trim, h, proof, mandatory, opts, o.MaxInputBits)
}

func tessellateExactTrimPartition(ctx context.Context, p *geom.SurfacePatch, trim SplineUV, h [3][3]*big.Rat, proof CertifiedMultiSpanTrimSeam, mandatory []*big.Rat, opts QuadraticTrimTessellationOptions, maxInputBits int) (CertifiedRationalCutTrimPolyline, error) {
	empty := CertifiedRationalCutTrimPolyline{}
	var e error
	blossom := func(a, b *big.Rat) [3]*big.Rat {
		ma, mb := new(big.Rat).Sub(big.NewRat(1, 1), a), new(big.Rat).Sub(big.NewRat(1, 1), b)
		weights := [3]*big.Rat{new(big.Rat).Mul(ma, mb), new(big.Rat).Add(new(big.Rat).Mul(a, mb), new(big.Rat).Mul(b, ma)), new(big.Rat).Mul(a, b)}
		var v [3]*big.Rat
		for k := range v {
			v[k] = new(big.Rat)
			for j, w := range weights {
				v[k].Add(v[k], new(big.Rat).Mul(w, h[j][k]))
			}
		}
		return v
	}
	type sample struct {
		point geom.Vec3
		exact [3]*big.Rat
		uv    [2]*big.Rat
	}
	cache := map[string]sample{}
	evaluate := func(q *big.Rat) (sample, error) {
		if s, ok := cache[q.RatString()]; ok {
			return s, nil
		}
		var s sample
		v := blossom(q, q)
		s.uv = [2]*big.Rat{new(big.Rat).Quo(v[0], v[2]), new(big.Rat).Quo(v[1], v[2])}
		jet, e := p.EvaluateRational(ctx, s.uv[0], s.uv[1], opts.Evaluation)
		if e != nil {
			return s, e
		}
		s.exact = jet.Point
		s.point.X, _ = jet.Point[0].Float64()
		s.point.Y, _ = jet.Point[1].Float64()
		s.point.Z, _ = jet.Point[2].Float64()
		if !s.point.IsFinite() {
			return s, geom.ErrConvergence
		}
		cache[q.RatString()] = s
		return s, nil
	}
	result := CertifiedRationalCutTrimPolyline{Source: trim, FixedCuts: mandatory, ParameterMap: proof.ParameterMaps[0]}
	appendPoint := func(q *big.Rat, s sample) {
		result.NormalizedParameters = append(result.NormalizedParameters, new(big.Rat).Set(q))
		result.Parameters = append(result.Parameters, new(big.Rat).Add(result.ParameterMap.Offset, new(big.Rat).Mul(result.ParameterMap.Scale, q)))
		result.Points = append(result.Points, s.point)
		result.ExactPoints = append(result.ExactPoints, s.exact)
		result.ExactUV = append(result.ExactUV, s.uv)
	}
	us, vs := p.KnotBreaks()
	cells := map[[2]int]*geom.SurfacePatch{}
	target := ratInset(opts.Deflection)
	target.Mul(target, target)
	type interval struct {
		lo, hi *big.Rat
		depth  int
	}
	pieceIndex := 0
	for fixed := 0; fixed+1 < len(mandatory); fixed++ {
		lo, hi := mandatory[fixed], mandatory[fixed+1]
		for pieceIndex+1 < len(proof.Pieces) && lo.Cmp(proof.Pieces[pieceIndex].Max) >= 0 {
			pieceIndex++
		}
		piece := proof.Pieces[pieceIndex]
		if lo.Cmp(piece.Min) < 0 || hi.Cmp(piece.Max) > 0 {
			return empty, geom.ErrConvergence
		}
		cellID := piece.Cells[0]
		cell, ok := cells[cellID]
		if !ok {
			cell, e = p.Trim(geom.ParameterRectangle{UMin: us[cellID[0]], UMax: us[cellID[0]+1], VMin: vs[cellID[1]], VMax: vs[cellID[1]+1]})
			if e != nil {
				return empty, e
			}
			cells[cellID] = cell
		}
		stack := []interval{{lo: lo, hi: hi}}
		for len(stack) > 0 {
			if e = ctx.Err(); e != nil {
				return empty, e
			}
			if result.Checks == opts.MaxChecks {
				return empty, geom.ErrConvergence
			}
			next := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			result.Checks++
			a, e := evaluate(next.lo)
			if e != nil {
				return empty, e
			}
			b, e := evaluate(next.hi)
			if e != nil {
				return empty, e
			}
			controls := [3][3]*big.Rat{blossom(next.lo, next.lo), blossom(next.lo, next.hi), blossom(next.hi, next.hi)}
			bound, e := boundExactQuadraticTrimControls(ctx, cell, controls, a.point, b.point)
			if e != nil && e != geom.ErrDomain {
				return empty, e
			}
			if e == nil && bound.Cmp(target) <= 0 {
				if len(result.SquaredErrors) == opts.MaxSegments {
					return empty, geom.ErrConvergence
				}
				if len(result.Points) == 0 {
					appendPoint(next.lo, a)
				}
				appendPoint(next.hi, b)
				result.SquaredErrors = append(result.SquaredErrors, bound)
				result.Cells = append(result.Cells, cellID)
				continue
			}
			if next.depth == opts.MaxDepth || len(result.SquaredErrors)+len(stack)+2+(len(mandatory)-fixed-2) > opts.MaxSegments {
				return empty, geom.ErrConvergence
			}
			mid := new(big.Rat).Quo(new(big.Rat).Add(next.lo, next.hi), big.NewRat(2, 1))
			if mid.Num().BitLen() > maxInputBits || mid.Denom().BitLen() > maxInputBits {
				return empty, geom.ErrConvergence
			}
			stack = append(stack, interval{lo: mid, hi: next.hi, depth: next.depth + 1}, interval{lo: next.lo, hi: mid, depth: next.depth + 1})
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}

func boundExactQuadraticTrimControls(ctx context.Context, p *geom.SurfacePatch, c [3][3]*big.Rat, a, b geom.Vec3) (*big.Rat, error) {
	d := p.Domain()
	minimum := new(big.Rat).Set(c[0][2])
	for _, h := range c {
		if h[2].Sign() <= 0 {
			return nil, geom.ErrInvalid
		}
		if h[2].Cmp(minimum) < 0 {
			minimum.Set(h[2])
		}
		for axis, ends := range [][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
			value := new(big.Rat).Quo(h[axis], h[2])
			if value.Cmp(ratInset(ends[0])) < 0 || value.Cmp(ratInset(ends[1])) > 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	var powers [3]trimPower9
	for axis := range powers {
		powers[axis] = trimZero9()
		powers[axis][0].Set(c[0][axis])
		powers[axis][1].Mul(big.NewRat(2, 1), new(big.Rat).Sub(c[1][axis], c[0][axis]))
		powers[axis][2].Sub(new(big.Rat).Add(c[0][axis], c[2][axis]), new(big.Rat).Mul(big.NewRat(2, 1), c[1][axis]))
	}
	return boundTrimPowerCorrespondence(ctx, p, powers, minimum, a, b)
}
