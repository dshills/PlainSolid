//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Restriction examines at most eight original spans, retaining each bounded
// source proof and one complete restriction comparison. No source is flattened.
func restrictMultiSpanMappedSource(ctx context.Context, q MultiSpanMappedSplineUV, a, b *big.Rat, o geom.SurfacePatchOptions) (PCurve, error) {
	if a == nil || b == nil || a.Sign() < 0 || a.Cmp(b) >= 0 || b.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	var parts []MappedSplineUV
	var stations []*big.Rat
	lo := new(big.Rat)
	for i, p := range q.parts {
		hi := big.NewRat(1, 1)
		if i < len(q.stations) {
			hi = q.stations[i]
		}
		left, right := a, b
		if left.Cmp(lo) < 0 {
			left = lo
		}
		if right.Cmp(hi) > 0 {
			right = hi
		}
		if left.Cmp(right) < 0 {
			width := new(big.Rat).Sub(hi, lo)
			child, e := restrictMappedSource(ctx, p, new(big.Rat).Quo(new(big.Rat).Sub(left, lo), width), new(big.Rat).Quo(new(big.Rat).Sub(right, lo), width), o)
			if e != nil {
				return nil, e
			}
			parts = append(parts, child)
			if right.Cmp(b) < 0 {
				stations = append(stations, new(big.Rat).Quo(new(big.Rat).Sub(right, a), new(big.Rat).Sub(b, a)))
			}
		}
		lo = hi
	}
	if len(parts) == 0 {
		return nil, geom.ErrInvalid
	}
	var result PCurve = parts[0]
	if len(parts) > 1 {
		source := parts[0].source
		last := parts[len(parts)-1].source
		if source.Reversed {
			source.Range.Min = last.Range.Min
		} else {
			source.Range.Max = last.Range.Max
		}
		weights := make([]*big.Rat, len(parts))
		for i, p := range parts {
			weights[i] = p.weight
		}
		out, e := NewMultiSpanMappedSplineUV(ctx, source, stations, weights, o)
		if e != nil {
			return nil, e
		}
		result = out
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
func splitMultiSpanMappedSource(ctx context.Context, q MultiSpanMappedSplineUV, t *big.Rat, a, b ExactBezierUV, o geom.SurfacePatchOptions) (PCurve, PCurve, error) {
	x, e := restrictMultiSpanMappedSource(ctx, q, new(big.Rat), t, o)
	if e != nil {
		return nil, nil, e
	}
	y, e := restrictMultiSpanMappedSource(ctx, q, t, big.NewRat(1, 1), o)
	if e != nil {
		return nil, nil, e
	}
	for i, c := range []PCurve{x, y} {
		actual, _, _ := mappedRepairSource(c)
		if e = proveMappedRepairCurve(ctx, actual, []ExactBezierUV{a, b}[i], o); e != nil {
			return nil, nil, e
		}
	}
	return x, y, nil
}
