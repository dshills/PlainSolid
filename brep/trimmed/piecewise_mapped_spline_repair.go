package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

func restrictMappedSource(ctx context.Context, q MappedSplineUV, a, b *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	var zero MappedSplineUV
	if a.Sign() < 0 || a.Cmp(b) >= 0 || b.Cmp(big.NewRat(1, 1)) > 0 {
		return zero, geom.ErrDomain
	}
	x, e := q.AuthoredParameter(a)
	if e != nil {
		return zero, e
	}
	y, e := q.AuthoredParameter(b)
	if e != nil {
		return zero, e
	}
	lo, e := geom.RepresentedExactScalar(x)
	if e != nil {
		return zero, e
	}
	hi, e := geom.RepresentedExactScalar(y)
	if e != nil {
		return zero, e
	}
	source := q.source
	if source.Reversed {
		lo, hi = hi, lo
	}
	source.Range = UVInterval{lo, hi}
	den := func(t *big.Rat) *big.Rat {
		return new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(new(big.Rat).Sub(q.weight, big.NewRat(1, 1)), t))
	}
	weight := new(big.Rat).Quo(den(b), den(a))
	out, e := NewMappedSplineUV(ctx, source, weight, o)
	if e != nil {
		return zero, e
	}
	expected, e := restrictMappedProofCurve(q.curve, a, b)
	if e != nil {
		return zero, e
	}
	if e = proveMappedRepairCurve(ctx, out.curve, expected, o); e != nil {
		return zero, e
	}
	return out, nil
}
func restrictMappedProofCurve(q ExactBezierUV, a, b *big.Rat) (ExactBezierUV, error) {
	var e error
	if b.Cmp(big.NewRat(1, 1)) < 0 {
		q, _, e = q.Split(b)
		if e != nil {
			return ExactBezierUV{}, e
		}
	}
	if a.Sign() > 0 {
		_, q, e = q.Split(new(big.Rat).Quo(a, b))
	}
	return q, e
}
func restrictPiecewiseMappedSource(ctx context.Context, q PiecewiseMappedSplineUV, a, b *big.Rat, o geom.SurfacePatchOptions) (PCurve, error) {
	if q.station == nil || a.Sign() < 0 || a.Cmp(b) >= 0 || b.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	var result PCurve
	if b.Cmp(q.station) <= 0 {
		p, e := restrictMappedSource(ctx, q.parts[0], new(big.Rat).Quo(a, q.station), new(big.Rat).Quo(b, q.station), o)
		if e != nil {
			return nil, e
		}
		result = p
	} else if a.Cmp(q.station) >= 0 {
		width := new(big.Rat).Sub(big.NewRat(1, 1), q.station)
		p, e := restrictMappedSource(ctx, q.parts[1], new(big.Rat).Quo(new(big.Rat).Sub(a, q.station), width), new(big.Rat).Quo(new(big.Rat).Sub(b, q.station), width), o)
		if e != nil {
			return nil, e
		}
		result = p
	} else {
		first, e := restrictMappedSource(ctx, q.parts[0], new(big.Rat).Quo(a, q.station), big.NewRat(1, 1), o)
		if e != nil {
			return nil, e
		}
		second, e := restrictMappedSource(ctx, q.parts[1], new(big.Rat), new(big.Rat).Quo(new(big.Rat).Sub(b, q.station), new(big.Rat).Sub(big.NewRat(1, 1), q.station)), o)
		if e != nil {
			return nil, e
		}
		source := first.source
		if source.Reversed {
			source.Range.Min = second.source.Range.Min
		} else {
			source.Range.Max = second.source.Range.Max
		}
		station := new(big.Rat).Quo(new(big.Rat).Sub(q.station, a), new(big.Rat).Sub(b, a))
		whole, e := NewPiecewiseMappedSplineUV(ctx, source, station, [2]*big.Rat{first.weight, second.weight}, o)
		if e != nil {
			return nil, e
		}
		result = whole
	}
	expected, e := restrictMappedProofCurve(q.curve, a, b)
	if e != nil {
		return nil, e
	}
	actual, _, ok := mappedRepairSource(result)
	if !ok {
		return nil, geom.ErrInvalid
	}
	if e = proveMappedRepairCurve(ctx, actual, expected, o); e != nil {
		return nil, e
	}
	return result, nil
}
func mappedRepairSource(c PCurve) (ExactBezierUV, SplineUV, bool) {
	switch q := c.(type) {
	case MappedSplineUV:
		return q.curve, q.source, q.weight != nil
	case PiecewiseMappedSplineUV:
		return q.curve, q.source, q.station != nil
	}
	return ExactBezierUV{}, SplineUV{}, false
}
func splitPiecewiseMappedSource(ctx context.Context, q PiecewiseMappedSplineUV, t *big.Rat, a, b ExactBezierUV, o geom.SurfacePatchOptions) (PCurve, PCurve, error) {
	x, e := restrictPiecewiseMappedSource(ctx, q, new(big.Rat), t, o)
	if e != nil {
		return nil, nil, e
	}
	y, e := restrictPiecewiseMappedSource(ctx, q, t, big.NewRat(1, 1), o)
	if e != nil {
		return nil, nil, e
	}
	for i, c := range []PCurve{x, y} {
		actual, _, _ := mappedRepairSource(c)
		if e := proveMappedRepairCurve(ctx, actual, []ExactBezierUV{a, b}[i], o); e != nil {
			return nil, nil, e
		}
	}
	return x, y, nil
}

type mappedRepairPiece struct {
	q      MappedSplineUV
	lo, hi *big.Rat
}

// At most four input pieces, three adjacent joins and one two-span rebuild are
// examined. Each source/curve reproof retains the bounded native merge budget;
// original source identity, raw adjacency and exact parameter maps are mandatory.
// Two single-span children may first attempt the existing bounded single-map
// join; its convergence refusal adds at most one such proof before this path.
func joinPiecewiseMappedSources(ctx context.Context, a, b PCurve, t *big.Rat) (PCurve, [2]geom.Vec2, error) {
	var zero [2]geom.Vec2
	ca, sa, oka := mappedRepairSource(a)
	cb, sb, okb := mappedRepairSource(b)
	if !oka || !okb {
		return nil, zero, geom.ErrInvalid
	}
	if !sameAuthoredBezierSpline(sa.Curve, sb.Curve) || sa.Reversed != sb.Reversed || sa.ExactEndpoints != sb.ExactEndpoints {
		return nil, zero, geom.ErrConvergence
	}
	source := sa
	if sa.Reversed {
		if sa.Range.Min != sb.Range.Max {
			return nil, zero, geom.ErrConvergence
		}
		source.Range.Min = sb.Range.Min
	} else {
		if sa.Range.Max != sb.Range.Min {
			return nil, zero, geom.ErrConvergence
		}
		source.Range.Max = sb.Range.Max
	}
	var parts []mappedRepairPiece
	appendParts := func(c PCurve, lo, hi *big.Rat) {
		switch q := c.(type) {
		case MappedSplineUV:
			parts = append(parts, mappedRepairPiece{q, lo, hi})
		case PiecewiseMappedSplineUV:
			mid := new(big.Rat).Add(lo, new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), q.station))
			parts = append(parts, mappedRepairPiece{q.parts[0], lo, mid}, mappedRepairPiece{q.parts[1], mid, hi})
		}
	}
	appendParts(a, new(big.Rat), t)
	appendParts(b, t, big.NewRat(1, 1))
	for i := 0; i+1 < len(parts); {
		x, y := parts[i], parts[i+1]
		raw := x.q.source.Range.Max
		if source.Reversed {
			raw = x.q.source.Range.Min
		}
		knot := false
		domainLo, domainHi := source.Curve.Domain()
		for _, k := range source.Curve.Knots() {
			if k > domainLo && k < domainHi && k == raw {
				knot = true
				break
			}
		}
		if knot {
			i++
			continue
		}
		fraction := new(big.Rat).Quo(new(big.Rat).Sub(x.hi, x.lo), new(big.Rat).Sub(y.hi, x.lo))
		joined, _, e := joinMappedSplineSources(ctx, x.q, y.q, fraction)
		if e != nil {
			return nil, zero, e
		}
		parts[i] = mappedRepairPiece{joined.(MappedSplineUV), x.lo, y.hi}
		parts = append(parts[:i+1], parts[i+2:]...)
	}
	o := geom.SurfacePatchOptions{MaxOperations: 1000000, MaxCoefficientBits: 16384}
	var result PCurve
	switch len(parts) {
	case 1:
		result = parts[0].q
	case 2:
		q, e := NewPiecewiseMappedSplineUV(ctx, source, parts[0].hi, [2]*big.Rat{parts[0].q.weight, parts[1].q.weight}, o)
		if e != nil {
			return nil, zero, e
		}
		result = q
	default:
		return nil, zero, geom.ErrConvergence
	}
	whole, _, _ := mappedRepairSource(result)
	x, y, e := whole.Split(t)
	if e != nil {
		return nil, zero, e
	}
	for i, c := range []ExactBezierUV{ca, cb} {
		if e := proveMappedRepairCurve(ctx, c, []ExactBezierUV{x, y}[i], o); e != nil {
			return nil, zero, e
		}
	}
	return result, whole.ends, nil
}
