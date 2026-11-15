package geom

import (
	"context"
	"math/big"
	"sort"
)

// RationalSegment extracts one exact Bezier curve between rational original
// parameters, mapping the result to [0,1]. start>end reverses orientation.
// Unlike Restrict, this operation need not encode rational cuts as float knots.
func (c *SurfaceIsoCurve) RationalSegment(ctx context.Context, start, end *big.Rat, o TubeAffineLineOptions) (*ExactSplineCurve, error) {
	if ctx == nil || c == nil || c.patch == nil || start == nil || end == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return nil, ErrInvalid
	}
	p := c.patch
	if p.countU != p.degreeU+1 {
		return nil, ErrConvergence
	}
	loDomain, hiDomain := c.Domain()
	origin, width := new(big.Rat).SetFloat64(loDomain), new(big.Rat).Sub(new(big.Rat).SetFloat64(hiDomain), new(big.Rat).SetFloat64(loDomain))
	lo, hi := new(big.Rat).Quo(new(big.Rat).Sub(start, origin), width), new(big.Rat).Quo(new(big.Rat).Sub(end, origin), width)
	reverse := lo.Cmp(hi) > 0
	if reverse {
		lo, hi = hi, lo
	}
	if lo.Sign() < 0 || hi.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, ErrDomain
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, x := range []*big.Rat{lo, hi} {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	net := make([]exactHomogeneous, p.countU)
	for i := range net {
		net[i] = p.net[i*p.countV]
	}
	restricted, e := restrictHomogeneousBezier(net, lo, hi, b)
	if e != nil {
		return nil, e
	}
	if reverse {
		for i, j := 0, len(restricted)-1; i < j; i, j = i+1, j-1 {
			restricted[i], restricted[j] = restricted[j], restricted[i]
		}
	}
	return exactCurveFromHomogeneousBezier(restricted, o, b)
}

// RationalSegmentPieces extracts a continuous directed segment from an exact
// model spline. The piece boundaries are normalized to [0,1] for the
// requested segment. Each piece records its original curve knot interval
// in Cell.UMin and Cell.UMax. Start and end need not have binary representations.
func (c *SurfaceIsoCurve) RationalSegmentPieces(ctx context.Context, start, end *big.Rat, o SurfaceAffineCurveOptions) (SurfaceAffineCurvePieces, error) {
	if c == nil || c.patch == nil || start == nil || end == nil || start.Cmp(end) == 0 {
		return SurfaceAffineCurvePieces{}, ErrInvalid
	}
	fixed := new(big.Rat).SetFloat64(c.patch.Domain().VMin)
	out, err := c.patch.AffineParameterCurvePieces(ctx, [2]*big.Rat{start, fixed}, [2]*big.Rat{end, fixed}, o)
	for i := range out.Pieces {
		out.Pieces[i].Cell.VMin = 0
		out.Pieces[i].Cell.VMax = 0
	}
	return out, err
}

// CompareAffineParameterCurvePieces compares exact piece collections over global
// normalized [0,1], using the union of their rational cut parameters. Each piece
// must contain a single Bezier curve on local [0,1]. ReverseB reverses the entire
// second collection. Public Complete/Continuous claims are not trusted: partition
// coverage is revalidated. Nonidentity does not imply geometric separation.
func CompareAffineParameterCurvePieces(ctx context.Context, a, b SurfaceAffineCurvePieces, o ExactCurveComparisonOptions) (r ExactCurveComparison, err error) {
	r.DifferentSpan, r.DifferentAxis = -1, -1
	if ctx == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxSpans == 0 {
		o.MaxSpans = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSpans < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	groups := [2][]SurfaceAffineCurvePiece{a.Pieces, b.Pieces}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for index, pieces := range groups {
		if len(pieces) == 0 {
			return r, ErrInvalid
		}
		last := new(big.Rat)
		for _, piece := range pieces {
			if piece.Start == nil || piece.End == nil || piece.Curve == nil || piece.Start.Cmp(last) != 0 || piece.End.Cmp(piece.Start) <= 0 {
				return r, ErrInvalid
			}
			lo, hi := piece.Curve.Domain()
			if lo != 0 || hi != 1 {
				return r, ErrInvalid
			}
			for _, t := range []*big.Rat{piece.Start, piece.End} {
				if e := budget.check(t); e != nil {
					return r, e
				}
				cut := new(big.Rat).Set(t)
				if index == 1 && o.ReverseB {
					cut.Sub(big.NewRat(1, 1), cut)
				}
				cuts = append(cuts, cut)
			}
			last = piece.End
		}
		if last.Cmp(big.NewRat(1, 1)) != 0 {
			return r, ErrInvalid
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	unique := cuts[:0]
	for _, cut := range cuts {
		if len(unique) == 0 || unique[len(unique)-1].Cmp(cut) != 0 {
			unique = append(unique, cut)
		}
	}
	cuts = unique
	if len(cuts)-1 > o.MaxSpans {
		return r, ErrConvergence
	}
	for i := 0; i < len(cuts)-1; i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		var curves [2]*ExactSplineCurve
		for group, pieces := range groups {
			start, end := new(big.Rat).Set(cuts[i]), new(big.Rat).Set(cuts[i+1])
			if group == 1 && o.ReverseB {
				start.Sub(big.NewRat(1, 1), start)
				end.Sub(big.NewRat(1, 1), end)
			}
			mid := new(big.Rat).Quo(new(big.Rat).Add(start, end), big.NewRat(2, 1))
			found := false
			for _, piece := range pieces {
				if mid.Cmp(piece.Start) <= 0 || mid.Cmp(piece.End) >= 0 {
					continue
				}
				width := new(big.Rat).Sub(piece.End, piece.Start)
				localStart := new(big.Rat).Quo(new(big.Rat).Sub(start, piece.Start), width)
				localEnd := new(big.Rat).Quo(new(big.Rat).Sub(end, piece.Start), width)
				remaining := budget.maxOperations - budget.operations
				if remaining < 1 {
					return r, ErrConvergence
				}
				curve, e := piece.Curve.RationalSegment(ctx, localStart, localEnd, TubeAffineLineOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxPoles: 8192})
				if e != nil {
					return r, e
				}
				budget.operations += curve.ExactOperations()
				curves[group] = curve
				found = true
				break
			}
			if !found {
				return r, ErrInvalid
			}
		}
		remaining := budget.maxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		comparison, e := CompareExactCurvesContext(ctx, curves[0], curves[1], ExactCurveComparisonOptions{MaxSpans: 1, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += comparison.ExactOperations
		if e != nil {
			return r, e
		}
		r.ComparedSpans++
		if !comparison.Equal {
			r.Complete = true
			r.DifferentSpan = i
			r.DifferentAxis = comparison.DifferentAxis
			return r, nil
		}
	}
	r.Equal = true
	r.Complete = true
	return r, nil
}
