package geom

import (
	"context"
	"math/big"
)

// CircularMeridianSegment describes a straight meridian boundary fragment.
// Points are (nonnegative radius, absolute height), not XY coordinates.
// Rotating the fragment about Center produces a disk, annulus, cylinder or
// conical band. ID is returned unchanged for the selected witness.
type CircularMeridianSegment struct {
	Start, End [2]*big.Rat
	ID         int
	Part       CylinderBoundaryPart
}

// CircularMeridianDistanceBounds encloses the minimum squared distance to
// the union of the supplied revolved segments. Witness is exact and belongs
// to the source segment at index Segment; Face retains its caller-supplied ID.
type CircularMeridianDistanceBounds struct {
	FrustumDistanceBounds
	Segment, Face int
}

// BoundCircularMeridianSegmentDistance computes global distance to a union
// of revolved straight fragments by reducing to one meridian plane. The
// radial square root is enclosed exactly. A rational witness is lifted into
// the original azimuth, including on a disk or annulus interior.
func BoundCircularMeridianSegmentDistance(ctx context.Context, center [2]*big.Rat, segments []CircularMeridianSegment, query Vec3,
	o FrustumDistanceOptions) (CircularMeridianDistanceBounds, error) {
	var empty CircularMeridianDistanceBounds
	if ctx == nil || !query.IsFinite() || center[0] == nil || center[1] == nil ||
		len(segments) == 0 || len(segments) > 512 || !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 ||
		o.MaxBisections < 0 || o.MaxBisections > 4096 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxBisections == 0 {
		o.MaxBisections = 256
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, s := range segments {
		for _, p := range [2][2]*big.Rat{s.Start, s.End} {
			if p[0] == nil || p[1] == nil || p[0].Sign() < 0 {
				return empty, ErrInvalid
			}
			for _, n := range p {
				if e := b.check(n); e != nil {
					return empty, e
				}
			}
		}
		if s.Start[0].Cmp(s.End[0]) == 0 && s.Start[1].Cmp(s.End[1]) == 0 {
			return empty, ErrDegenerate
		}
	}
	x := new(big.Rat).Sub(new(big.Rat).SetFloat64(query.X), center[0])
	y := new(big.Rat).Sub(new(big.Rat).SetFloat64(query.Y), center[1])
	z := new(big.Rat).SetFloat64(query.Z)
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	tol := new(big.Rat).SetFloat64(o.SquaredTolerance)
	for _, n := range []*big.Rat{x, y, z, q, tol} {
		if e := b.check(n); e != nil {
			return empty, e
		}
	}
	for bits := min(o.MaxBisections, 8); ; bits = min(o.MaxBisections, bits+8) {
		lo, hi, e := rationalSqrtBounds(q, bits, b)
		if e != nil {
			return empty, e
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		left, right, middle := distanceMeridianPoint{lo, z}, distanceMeridianPoint{hi, z}, distanceMeridianPoint{mid, z}
		var result CircularMeridianDistanceBounds
		var winner distanceMeridianPoint
		for i, s := range segments {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			a, c := distanceMeridianPoint(s.Start), distanceMeridianPoint(s.End)
			lower := meridianSegmentDistance(left, right, a, c)
			point, _ := meridianProjection(middle, a, c)
			upper := meridianSquared(left, point)
			if u := meridianSquared(right, point); u.Cmp(upper) > 0 {
				upper = u
			}
			for _, n := range []*big.Rat{lower, upper, point[0], point[1]} {
				if e := b.check(n); e != nil {
					return empty, e
				}
			}
			if result.Lower == nil || lower.Cmp(result.Lower) < 0 {
				result.Lower = lower
			}
			if result.Upper == nil || upper.Cmp(result.Upper) < 0 {
				result.Upper, result.Segment, result.Face, winner = upper, i, s.ID, point
			}
		}
		if new(big.Rat).Sub(result.Upper, result.Lower).Cmp(tol) <= 0 {
			axis := q.Sign() == 0
			if axis {
				x, q = big.NewRat(1, 1), big.NewRat(1, 1)
			}
			result.Bisections = bits
			result.Witness = FrustumDistanceWitness{
				CenterX: new(big.Rat).Set(center[0]), CenterY: new(big.Rat).Set(center[1]),
				Radius: winner[0], Height: winner[1], AzimuthX: x, AzimuthY: y, AzimuthSquared: q,
				Part: segments[result.Segment].Part, RotationallyNonunique: axis && winner[0].Sign() > 0,
			}
			return result, nil
		}
		if bits == o.MaxBisections {
			return empty, ErrConvergence
		}
	}
}
