package offset

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FootprintLineParameter encloses one exact line parameter. Min equals Max
// for an exactly rational value. Parameters retain the supplied direction's
// scale. Edges and Corners identify original boundary owners at a contact.
type FootprintLineParameter struct {
	Min, Max       *big.Rat
	Edges, Corners []int
}

// FootprintLineSpan is a connected finite line interval. Interior spans have
// open material ends. Boundary spans are closed and retain original edge IDs.
type FootprintLineSpan struct {
	Start, End FootprintLineParameter
	Edges      []int
}

// FootprintLineClip reports the original ideal footprint's isolated boundary
// contacts and finite interior or boundary spans. A supporting line can share
// a closed straight edge without entering the interior.
type FootprintLineClip struct {
	Hits               []FootprintLineParameter
	Interior, Boundary []FootprintLineSpan
}

// ClipLineRational intersects a finite local section line with the certified
// ideal rounded footprint. It verifies finite edge and arc-sector ownership
// exactly. The positive requested parameter width controls algebraic brackets.
// Intervals must separate within 1024 bits of radical refinement. A failure
// returns no partial clip. Input coefficient magnitudes are capped at 16384 bits.
func (q ConvexFootprintDilation) ClipLineRational(ctx context.Context, origin, direction [2]*big.Rat, lower, upper, tolerance *big.Rat) (FootprintLineClip, error) {
	var empty FootprintLineClip
	if ctx == nil || q.source == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	for _, v := range []*big.Rat{origin[0], origin[1], direction[0], direction[1], lower, upper, tolerance} {
		if v == nil {
			return empty, geom.ErrInvalid
		}
		if v.Num().BitLen() > 16384 || v.Denom().BitLen() > 16384 {
			return empty, ErrLimit
		}
	}
	if lower.Cmp(upper) >= 0 || tolerance.Sign() <= 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	o, d := pt{origin[0], origin[1]}, pt{direction[0], direction[1]}
	var candidates []footprintLineRoot
	var result FootprintLineClip
	for _, primitive := range q.source.primitives {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if primitive.Radius == 0 {
			candidate, boundary := footprintLineSegment(primitive, o, d, lower, upper)
			if candidate != nil {
				candidates = append(candidates, *candidate)
			}
			if boundary != nil {
				result.Boundary = append(result.Boundary, *boundary)
			}
			continue
		}
		candidates = append(candidates, footprintLineArc(primitive, o, d, lower, upper)...)
	}
	var err error
	result.Hits, err = q.separateFootprintRoots(ctx, candidates, o, d, lower, upper, tolerance)
	if err != nil {
		return empty, err
	}
	if len(result.Boundary) != 0 {
		for i := range result.Boundary {
			span := &result.Boundary[i]
			for _, endpoint := range []*FootprintLineParameter{&span.Start, &span.End} {
				p := footprintLinePoint(o, d, endpoint.Min)
				owners, err := q.LocateRational(ctx, [2]*big.Rat{p.x, p.y})
				if err != nil {
					return empty, err
				}
				if owners.Location != parametric.Boundary {
					return empty, geom.ErrConvergence
				}
				endpoint.Edges, endpoint.Corners = owners.Edges, owners.Corners
			}
		}
		return result, nil
	}
	ordered := []FootprintLineParameter{footprintLineEndpoint(lower)}
	for _, hit := range result.Hits {
		if hit.Min.Cmp(lower) > 0 && hit.Max.Cmp(upper) < 0 {
			ordered = append(ordered, hit)
		} else if hit.Min.Cmp(lower) == 0 && hit.Max.Cmp(lower) == 0 {
			ordered[0] = hit
		}
	}
	last := footprintLineEndpoint(upper)
	if len(result.Hits) != 0 {
		hit := result.Hits[len(result.Hits)-1]
		if hit.Min.Cmp(upper) == 0 && hit.Max.Cmp(upper) == 0 {
			last = hit
		}
	}
	ordered = append(ordered, last)
	for i := 0; i+1 < len(ordered); i++ {
		a, b := ordered[i], ordered[i+1]
		mid := mid(a.Max, b.Min)
		p := footprintLinePoint(o, d, mid)
		check, err := q.LocateRational(ctx, [2]*big.Rat{p.x, p.y})
		if err != nil {
			return empty, err
		}
		if check.Location == parametric.Boundary {
			return empty, geom.ErrConvergence
		}
		if check.Location == parametric.Inside {
			result.Interior = append(result.Interior, FootprintLineSpan{Start: a, End: b})
		}
	}
	return result, nil
}

type footprintLineRoot struct {
	base, factor, radical *big.Rat
	corner                int
}

func (q footprintLineRoot) compare(v *big.Rat) int {
	b := sub(q.base, v)
	if q.factor == nil || q.factor.Sign() == 0 {
		return b.Sign()
	}
	x, y := b.Sign(), q.factor.Sign()
	if x == 0 || x == y {
		return y
	}
	return x * mul(b, b).Cmp(mul(mul(q.factor, q.factor), q.radical))
}

func (q footprintLineRoot) affineSign(a, b *big.Rat) int {
	shift := add(a, mul(b, q.base))
	if q.factor == nil {
		return shift.Sign()
	}
	return (footprintLineRoot{base: shift, factor: mul(b, q.factor), radical: q.radical}).compare(new(big.Rat))
}

func (q footprintLineRoot) bounds(bits uint) (lo, hi *big.Rat) {
	if q.factor == nil {
		return new(big.Rat).Set(q.base), new(big.Rat).Set(q.base)
	}
	denom := new(big.Int).Lsh(big.NewInt(1), bits)
	n := new(big.Int).Lsh(q.radical.Num(), 2*bits)
	n.Quo(n, q.radical.Denom()).Sqrt(n)
	loRoot := new(big.Rat).SetFrac(n, denom)
	hiRoot := new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), denom)
	lo, hi = add(q.base, mul(q.factor, loRoot)), add(q.base, mul(q.factor, hiRoot))
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	return lo, hi
}

func footprintLineSegment(primitive ConvexOffsetPrimitive, origin, direction pt, lo, hi *big.Rat) (*footprintLineRoot, *FootprintLineSpan) {
	a, b := footprintPoint(primitive.Start), footprintPoint(primitive.End)
	edge, delta := footprintDifference(b, a), footprintDifference(a, origin)
	denom := footprintCross(direction, edge)
	if denom.Sign() != 0 {
		t, s := footprintCross(delta, edge), footprintCross(delta, direction)
		t.Quo(t, denom)
		s.Quo(s, denom)
		if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 || s.Sign() < 0 || s.Cmp(big.NewRat(1, 1)) > 0 {
			return nil, nil
		}
		return &footprintLineRoot{base: t, corner: -1}, nil
	}
	if footprintCross(delta, direction).Sign() != 0 {
		return nil, nil
	}
	parameter := func(p pt) *big.Rat {
		r := footprintDifference(p, origin)
		if direction.x.Sign() != 0 {
			return new(big.Rat).Quo(r.x, direction.x)
		}
		return new(big.Rat).Quo(r.y, direction.y)
	}
	x, y := parameter(a), parameter(b)
	if x.Cmp(y) > 0 {
		x, y = y, x
	}
	if x.Cmp(lo) < 0 {
		x = lo
	}
	if y.Cmp(hi) > 0 {
		y = hi
	}
	if x.Cmp(y) > 0 {
		return nil, nil
	}
	if x.Cmp(y) == 0 {
		return &footprintLineRoot{base: x, corner: -1}, nil
	}
	return nil, &FootprintLineSpan{Start: footprintLineEndpoint(x), End: footprintLineEndpoint(y), Edges: []int{primitive.OriginalEdge}}
}

func footprintLineArc(primitive ConvexOffsetPrimitive, origin, direction pt, lo, hi *big.Rat) []footprintLineRoot {
	center := footprintPoint(primitive.Center)
	p := footprintDifference(origin, center)
	a := footprintDot(direction, direction)
	b := mul(big.NewRat(2, 1), footprintDot(p, direction))
	radius := num(primitive.Radius)
	c := sub(footprintDot(p, p), mul(radius, radius))
	disc := sub(mul(b, b), mul(big.NewRat(4, 1), mul(a, c)))
	if disc.Sign() < 0 {
		return nil
	}
	den := mul(big.NewRat(2, 1), a)
	base := new(big.Rat).Quo(new(big.Rat).Neg(b), den)
	roots := []footprintLineRoot{{base: base, corner: primitive.OriginalVertex}}
	if disc.Sign() > 0 {
		q := new(big.Rat).Inv(den)
		if exact, ok := footprintSquareRoot(disc); ok {
			delta := mul(q, exact)
			roots[0].base = sub(base, delta)
			roots = append(roots, footprintLineRoot{base: add(base, delta), corner: primitive.OriginalVertex})
		} else {
			roots[0].factor, roots[0].radical = new(big.Rat).Neg(q), disc
			roots = append(roots, footprintLineRoot{base: base, factor: q, radical: disc, corner: primitive.OriginalVertex})
		}
	}
	start := footprintDifference(footprintPoint(primitive.Start), center)
	end := footprintDifference(footprintPoint(primitive.End), center)
	return slices.DeleteFunc(roots, func(root footprintLineRoot) bool {
		return root.compare(lo) < 0 || root.compare(hi) > 0 ||
			root.affineSign(footprintCross(start, p), footprintCross(start, direction)) < 0 ||
			root.affineSign(footprintCross(p, end), footprintCross(direction, end)) < 0
	})
}

func (q ConvexFootprintDilation) separateFootprintRoots(ctx context.Context, roots []footprintLineRoot, origin, direction pt, lower, upper, tolerance *big.Rat) ([]FootprintLineParameter, error) {
	var exact []FootprintLineParameter
	var irrational []footprintLineRoot
	seen := make(map[string]bool)
	for _, root := range roots {
		if root.factor != nil {
			irrational = append(irrational, root)
			continue
		}
		key := root.base.RatString()
		if seen[key] {
			continue
		}
		seen[key] = true
		p := footprintLinePoint(origin, direction, root.base)
		owners, err := q.LocateRational(ctx, [2]*big.Rat{p.x, p.y})
		if err != nil {
			return nil, err
		}
		if owners.Location != parametric.Boundary {
			return nil, geom.ErrConvergence
		}
		hit := footprintLineEndpoint(root.base)
		hit.Edges, hit.Corners = owners.Edges, owners.Corners
		exact = append(exact, hit)
	}
	for precision := uint(64); precision <= 1024; precision *= 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out := append([]FootprintLineParameter(nil), exact...)
		complete := true
		for _, root := range irrational {
			lo, hi := root.bounds(precision)
			if lo.Cmp(lower) <= 0 || hi.Cmp(upper) >= 0 || sub(hi, lo).Cmp(tolerance) > 0 {
				complete = false
				break
			}
			out = append(out, FootprintLineParameter{Min: lo, Max: hi, Corners: []int{root.corner}})
		}
		if !complete {
			continue
		}
		slices.SortFunc(out, func(a, b FootprintLineParameter) int { return a.Min.Cmp(b.Min) })
		for i := 1; i < len(out); i++ {
			if out[i-1].Max.Cmp(out[i].Min) >= 0 {
				complete = false
				break
			}
		}
		if complete {
			return out, nil
		}
	}
	return nil, ErrLimit
}

func footprintPoint(p geom.Vec2) pt { return pt{num(p.X), num(p.Y)} }
func footprintLinePoint(o, d pt, t *big.Rat) pt {
	return pt{add(o.x, mul(d.x, t)), add(o.y, mul(d.y, t))}
}
func footprintLineEndpoint(t *big.Rat) FootprintLineParameter {
	return FootprintLineParameter{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t)}
}
