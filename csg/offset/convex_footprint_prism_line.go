package offset

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FootprintPrismLineParameter adds the axial cap owners of an original
// ideal prism contact. Cap zero is the lower local plane and cap one the upper.
// Side owners retain the original profile's edge and corner indices.
type FootprintPrismLineParameter struct {
	FootprintLineParameter
	Caps []int
}

// FootprintPrismLineSpan has original support owners where it lies entirely on
// the boundary. Interior spans carry only their endpoint ownership.
type FootprintPrismLineSpan struct {
	Start, End           FootprintPrismLineParameter
	Caps, Edges, Corners []int
}

// FootprintPrismLineClip separates isolated hits, open volume intervals and
// closed coincident face intervals on the ideal rounded prism.
type FootprintPrismLineClip struct {
	Hits               []FootprintPrismLineParameter
	Interior, Boundary []FootprintPrismLineSpan
}

// ClipPrismLineRational intersects a finite model-space line with the ideal
// rounded prism. The proper signed frame is inverted exactly and the supplied
// line parameter is preserved. Cap planes, generators and rim ties are
// explicit. Algebraic side intervals obey the same limits as ClipLineRational.
func (q ConvexFootprintDilation) ClipPrismLineRational(ctx context.Context, origin, direction [3]*big.Rat, lower, upper, tolerance *big.Rat) (FootprintPrismLineClip, error) {
	var empty FootprintPrismLineClip
	if ctx == nil || q.source == nil || lower == nil || upper == nil || tolerance == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	for _, v := range []*big.Rat{origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], lower, upper, tolerance} {
		if v == nil {
			return empty, geom.ErrInvalid
		}
		if v.Num().BitLen() > 16384 || v.Denom().BitLen() > 16384 {
			return empty, ErrLimit
		}
	}
	if lower.Cmp(upper) >= 0 || tolerance.Sign() <= 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	var o, d [3]*big.Rat
	for i, row := range q.source.placement {
		for j, coefficient := range row[:3] {
			if coefficient != 0 {
				o[j] = mul(sub(origin[i], num(row[3])), num(coefficient))
				d[j] = mul(direction[i], num(coefficient))
			}
		}
	}
	z := q.source.z
	var capAt [2]*big.Rat
	var capPlane []int
	lo, hi := lower, upper
	if d[2].Sign() == 0 {
		if o[2].Cmp(z[0]) < 0 || o[2].Cmp(z[1]) > 0 {
			return empty, nil
		}
		for i, bound := range z {
			if o[2].Cmp(bound) == 0 {
				capPlane = []int{i}
			}
		}
	} else {
		for i, bound := range z {
			capAt[i] = new(big.Rat).Quo(sub(bound, o[2]), d[2])
		}
		x, y := capAt[0], capAt[1]
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		if x.Cmp(lo) > 0 {
			lo = x
		}
		if y.Cmp(hi) < 0 {
			hi = y
		}
		if lo.Cmp(hi) > 0 {
			return empty, nil
		}
	}
	decorate := func(hit FootprintLineParameter) FootprintPrismLineParameter {
		out := FootprintPrismLineParameter{FootprintLineParameter: hit, Caps: slices.Clone(capPlane)}
		if hit.Min.Cmp(hit.Max) == 0 {
			for i, t := range capAt {
				if t != nil && hit.Min.Cmp(t) == 0 {
					out.Caps = append(out.Caps, i)
				}
			}
		}
		return out
	}
	projection := func(t *big.Rat) (ConvexFootprintPoint, error) {
		return q.LocateRational(ctx, [2]*big.Rat{add(o[0], mul(d[0], t)), add(o[1], mul(d[1], t))})
	}
	var result FootprintPrismLineClip
	pointHit := func(t *big.Rat, p ConvexFootprintPoint) FootprintPrismLineParameter {
		hit := footprintLineEndpoint(t)
		hit.Edges, hit.Corners = slices.Clone(p.Edges), slices.Clone(p.Corners)
		return decorate(hit)
	}
	if lo.Cmp(hi) == 0 || d[0].Sign() == 0 && d[1].Sign() == 0 {
		where, err := projection(lo)
		if err != nil || where.Location == parametric.Outside {
			return empty, err
		}
		a, b := pointHit(lo, where), pointHit(hi, where)
		for i, hit := range [2]FootprintPrismLineParameter{a, b} {
			if (i == 0 || lo.Cmp(hi) != 0) && len(hit.Caps) != 0 {
				result.Hits = append(result.Hits, hit)
			}
		}
		if lo.Cmp(hi) == 0 {
			return result, nil
		}
		span := FootprintPrismLineSpan{Start: a, End: b}
		if where.Location == parametric.Boundary {
			span.Edges, span.Corners = slices.Clone(where.Edges), slices.Clone(where.Corners)
			result.Boundary = []FootprintPrismLineSpan{span}
		} else {
			result.Interior = []FootprintPrismLineSpan{span}
		}
		return result, nil
	}
	section, err := q.ClipLineRational(ctx, [2]*big.Rat{o[0], o[1]}, [2]*big.Rat{d[0], d[1]}, lo, hi, tolerance)
	if err != nil {
		return empty, err
	}
	for _, hit := range section.Hits {
		result.Hits = append(result.Hits, decorate(hit))
	}
	for _, cap := range capAt {
		if cap == nil || cap.Cmp(lo) < 0 || cap.Cmp(hi) > 0 || slices.ContainsFunc(result.Hits, func(h FootprintPrismLineParameter) bool {
			return h.Min.Cmp(cap) == 0 && h.Max.Cmp(cap) == 0
		}) {
			continue
		}
		p, err := projection(cap)
		if err != nil {
			return empty, err
		}
		if p.Location != parametric.Outside {
			result.Hits = append(result.Hits, pointHit(cap, p))
		}
	}
	slices.SortFunc(result.Hits, func(a, b FootprintPrismLineParameter) int { return a.Min.Cmp(b.Min) })
	lift := func(span FootprintLineSpan) FootprintPrismLineSpan {
		return FootprintPrismLineSpan{Start: decorate(span.Start), End: decorate(span.End),
			Caps: slices.Clone(capPlane), Edges: slices.Clone(span.Edges)}
	}
	for _, span := range section.Interior {
		if len(capPlane) == 0 {
			result.Interior = append(result.Interior, lift(span))
		} else {
			result.Boundary = append(result.Boundary, lift(span))
		}
	}
	for _, span := range section.Boundary {
		result.Boundary = append(result.Boundary, lift(span))
	}
	return result, nil
}
