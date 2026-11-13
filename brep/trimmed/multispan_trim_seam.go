package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// MultiSpanTrimSeamOptions bounds knot tests, paired pieces and exact identity
// evaluations. Defaults are 4096, 256 and 8192; maxima are 1M, 4096 and 131072. Evaluation bounds
// the cumulative exact chart evaluations. Irrational quadratic knot crossings
// are unresolved.
type MultiSpanTrimSeamOptions struct {
	MaxKnotTests, MaxPieces, MaxIdentityChecks int
	Evaluation                                 geom.SurfacePatchEvaluationOptions
}

// MultiSpanTrimSeamPiece covers a closed exact normalized interval on both
// directed original trims. SourceRanges retain original parameters, including
// reversed order. Cell indices refer to each support's own knot breaks.
type MultiSpanTrimSeamPiece struct {
	Min, Max             *big.Rat
	Cells                [2][2]int
	SourceRanges         [2][2]*big.Rat
	ExactStart, ExactEnd [3]*big.Rat
}

// CertifiedMultiSpanTrimSeam proves continuous same-normalized-parameter model
// equality across differing support knot partitions. It is not a polyline or
// mesh-distance certificate. Rational crossing parameters remain exact, even
// when not representable in binary64. No approximate knot snapping is used.
type CertifiedMultiSpanTrimSeam struct {
	ParameterMaps             [2]ExactTrimParameterMap
	Pieces                    []MultiSpanTrimSeamPiece
	KnotTests, IdentityChecks int
}

// CertifyMultiSpanTrimSeam finds every rational knot crossing of each directed
// positive rational linear/quadratic UV trim, forms one common partition, then
// proves composed homogeneous cross-polynomial equality on each paired cell.
// The composed degree bounds determine how many exact distinct
// evaluations prove identity (at least 17 per piece, at most 129).
// C0 continuity extends the result to shared break endpoints.
// Nonrational quadratic crossings, unsupported degree, mismatch or exhausted
// limits return no partial proof. Coincident knot traces are accepted; crossing
// an arbitrary smooth algebraic trim or nonlinear reparameterization is not.
func CertifyMultiSpanTrimSeam(ctx context.Context, a, b *geom.SurfacePatch, ta, tb SplineUV, o MultiSpanTrimSeamOptions) (CertifiedMultiSpanTrimSeam, error) {
	var controls [2][3][3]*big.Rat
	var maps [2]ExactTrimParameterMap
	for chart, trim := range [2]SplineUV{ta, tb} {
		h, e := quadraticTrimSourceControls(trim)
		if e != nil {
			return CertifiedMultiSpanTrimSeam{}, e
		}
		controls[chart] = h
		lo, hi := ratInset(trim.Range.Min), ratInset(trim.Range.Max)
		if trim.Reversed {
			lo, hi = hi, lo
		}
		maps[chart] = ExactTrimParameterMap{Offset: lo, Scale: new(big.Rat).Sub(hi, lo), Degree: trim.Curve.Degree()}
	}
	return certifyMultiSpanTrimControls(ctx, a, b, controls, maps, o)
}

func certifyMultiSpanTrimControls(ctx context.Context, a, b *geom.SurfacePatch, h [2][3][3]*big.Rat,
	maps [2]ExactTrimParameterMap, o MultiSpanTrimSeamOptions) (CertifiedMultiSpanTrimSeam, error) {
	empty := CertifiedMultiSpanTrimSeam{}
	if ctx == nil || a == nil || b == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxKnotTests == 0 {
		o.MaxKnotTests = 4096
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 256
	}
	if o.MaxIdentityChecks == 0 {
		o.MaxIdentityChecks = 8192
	}
	if o.MaxKnotTests < 1 || o.MaxKnotTests > 1000000 || o.MaxPieces < 1 || o.MaxPieces > 4096 || o.MaxIdentityChecks < 1 || o.MaxIdentityChecks > 131072 {
		return empty, geom.ErrInvalid
	}
	patches := [2]*geom.SurfacePatch{a, b}
	var cuts [2][2][]float64
	result := CertifiedMultiSpanTrimSeam{ParameterMaps: maps}
	left := o.Evaluation.MaxOperations
	if left == 0 {
		left = 8000000
	}
	if left < 1 {
		return empty, geom.ErrInvalid
	}
	breaks := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	evaluateUV := func(chart int, t *big.Rat) [2]*big.Rat {
		one := new(big.Rat).Sub(big.NewRat(1, 1), t)
		weights := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, t)), new(big.Rat).Mul(t, t)}
		var v [3]*big.Rat
		for k := range v {
			v[k] = new(big.Rat)
			for i, w := range weights {
				v[k].Add(v[k], new(big.Rat).Mul(w, h[chart][i][k]))
			}
		}
		return [2]*big.Rat{new(big.Rat).Quo(v[0], v[2]), new(big.Rat).Quo(v[1], v[2])}
	}
	for chart, p := range patches {
		d := p.Domain()
		if !finite(d.UMin) || !finite(d.UMax) || !finite(d.VMin) || !finite(d.VMax) {
			return empty, geom.ErrInvalid
		}
		for _, control := range h[chart] {
			u, v := new(big.Rat).Quo(control[0], control[2]), new(big.Rat).Quo(control[1], control[2])
			if u.Cmp(ratInset(d.UMin)) < 0 || u.Cmp(ratInset(d.UMax)) > 0 || v.Cmp(ratInset(d.VMin)) < 0 || v.Cmp(ratInset(d.VMax)) > 0 {
				return empty, geom.ErrDomain
			}
		}
		cuts[chart][0], cuts[chart][1] = p.KnotBreaks()
		if len(cuts[chart][0]) < 2 || len(cuts[chart][1]) < 2 {
			return empty, geom.ErrInvalid
		}

		for axis := 0; axis < 2; axis++ {
			for _, knot := range cuts[chart][axis][1 : len(cuts[chart][axis])-1] {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				if result.KnotTests == o.MaxKnotTests {
					return empty, geom.ErrConvergence
				}
				result.KnotTests++
				var bernstein [3]*big.Rat
				positive, negative := true, true
				for i := range bernstein {
					bernstein[i] = new(big.Rat).Sub(h[chart][i][axis], new(big.Rat).Mul(ratInset(knot), h[chart][i][2]))
					positive = positive && bernstein[i].Sign() > 0
					negative = negative && bernstein[i].Sign() < 0
				}
				if positive || negative {
					continue
				}
				coefficients := [3]*big.Rat{bernstein[0], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(bernstein[1], bernstein[0])), new(big.Rat).Add(new(big.Rat).Sub(bernstein[0], new(big.Rat).Mul(big.NewRat(2, 1), bernstein[1])), bernstein[2])}
				roots, e := rationalQuadraticSeamRoots(coefficients)
				if e != nil {
					return empty, e
				}
				for _, root := range roots {
					if root.Sign() > 0 && root.Cmp(big.NewRat(1, 1)) < 0 {
						breaks = append(breaks, root)
					}
				}
				if len(breaks) > 2+4*o.MaxPieces {
					return empty, geom.ErrConvergence
				}
			}
		}
	}
	sort.Slice(breaks, func(i, j int) bool { return breaks[i].Cmp(breaks[j]) < 0 })
	unique := breaks[:0]
	for _, q := range breaks {
		if len(unique) == 0 || q.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, q)
		}
	}
	breaks = unique
	if len(breaks)-1 > o.MaxPieces {
		return empty, geom.ErrConvergence
	}
	for piece := 0; piece+1 < len(breaks); piece++ {
		lo, hi := breaks[piece], breaks[piece+1]
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		out := MultiSpanTrimSeamPiece{Min: new(big.Rat).Set(lo), Max: new(big.Rat).Set(hi)}
		degree := 0
		for chart, p := range patches {
			uv := evaluateUV(chart, mid)
			indices := [2]int{-1, -1}
			for axis := 0; axis < 2; axis++ {
				for i := 0; i+1 < len(cuts[chart][axis]); i++ {
					if uv[axis].Cmp(ratInset(cuts[chart][axis][i])) >= 0 && uv[axis].Cmp(ratInset(cuts[chart][axis][i+1])) <= 0 {
						indices[axis] = i
						break
					}
				}
			}
			if indices[0] < 0 || indices[1] < 0 {
				return empty, geom.ErrDomain
			}
			cell, e := p.Trim(geom.ParameterRectangle{UMin: cuts[chart][0][indices[0]], UMax: cuts[chart][0][indices[0]+1], VMin: cuts[chart][1][indices[1]], VMax: cuts[chart][1][indices[1]+1]})
			if e != nil {
				return empty, e
			}
			u, v := cell.KnotBreaks()
			if len(u) != 2 || len(v) != 2 {
				return empty, geom.ErrConvergence
			}
			nu, nv := cell.ControlShape()
			degree += 2 * (nu + nv - 2)
			out.Cells[chart] = indices
			for i, q := range []*big.Rat{lo, hi} {
				out.SourceRanges[chart][i] = new(big.Rat).Add(result.ParameterMaps[chart].Offset, new(big.Rat).Mul(result.ParameterMaps[chart].Scale, q))
			}
		}
		degree = max(degree, 16)
		if degree > 128 || degree+1 > o.MaxIdentityChecks-result.IdentityChecks {
			return empty, geom.ErrConvergence
		}
		for i := 0; i <= degree; i++ {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			q := new(big.Rat).Add(lo, new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), big.NewRat(int64(i), int64(degree))))
			var model [2][3]*big.Rat
			for chart, p := range patches {
				uv := evaluateUV(chart, q)
				if left < 1 {
					return empty, geom.ErrConvergence
				}
				limits := o.Evaluation
				limits.MaxOperations = left
				jet, e := p.EvaluateRational(ctx, uv[0], uv[1], limits)
				if e != nil {
					return empty, e
				}
				left -= jet.ExactOperations
				model[chart] = jet.Point
			}
			result.IdentityChecks++
			for k := 0; k < 3; k++ {
				if model[0][k].Cmp(model[1][k]) != 0 {
					return empty, geom.ErrInvalid
				}
			}
			if i == 0 {
				out.ExactStart = model[0]
			}
			if i == degree {
				out.ExactEnd = model[0]
			}
		}
		result.Pieces = append(result.Pieces, out)
	}
	return result, nil
}

// Complete exact real roots when the quadratic discriminant has a rational
// square root. Identically zero means a trace on a knot, requiring no cuts.
func rationalQuadraticSeamRoots(c [3]*big.Rat) ([]*big.Rat, error) {
	if c[2].Sign() == 0 {
		if c[1].Sign() == 0 {
			return nil, nil
		}
		return []*big.Rat{new(big.Rat).Quo(new(big.Rat).Neg(c[0]), c[1])}, nil
	}
	disc := new(big.Rat).Sub(new(big.Rat).Mul(c[1], c[1]), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(c[0], c[2])))
	if disc.Sign() < 0 {
		return nil, nil
	}
	num, den := new(big.Int).Sqrt(disc.Num()), new(big.Int).Sqrt(disc.Denom())
	if new(big.Int).Mul(num, num).Cmp(disc.Num()) != 0 || new(big.Int).Mul(den, den).Cmp(disc.Denom()) != 0 {
		return nil, geom.ErrConvergence
	}
	root := new(big.Rat).SetFrac(num, den)
	minus := new(big.Rat).Neg(c[1])
	divisor := new(big.Rat).Mul(big.NewRat(2, 1), c[2])
	a := new(big.Rat).Quo(new(big.Rat).Sub(minus, root), divisor)
	b := new(big.Rat).Quo(new(big.Rat).Add(minus, root), divisor)
	if a.Cmp(b) == 0 {
		return []*big.Rat{a}, nil
	}
	return []*big.Rat{a, b}, nil
}
