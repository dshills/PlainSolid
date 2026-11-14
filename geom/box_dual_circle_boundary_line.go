package geom

import (
	"context"
	"math/big"
)

// BoxDualCircleSupport records exposed original supports in the local box
// frame. A stock axis identifies its minimum or maximum face by the contact
// point. Circle bits retain the original input order.
type BoxDualCircleSupport uint8

const (
	BoxDualCircleStockX BoxDualCircleSupport = 1 << iota
	BoxDualCircleStockY
	BoxDualCircleStockZ
	BoxDualCircleFirstWall
	BoxDualCircleSecondWall
	BoxDualCircleBlindFloor
)

type BoxDualCircleLineHit struct {
	Parameter    *ExactAlgebraicRoot
	Bounds       ExactParameterInterval
	Support      BoxDualCircleSupport
	Multiplicity int
}

type BoxDualCircleLineInterval struct {
	Start, End                        *ExactAlgebraicRoot
	StartBounds, EndBounds            ExactParameterInterval
	Support, StartSupport, EndSupport BoxDualCircleSupport
}

// BoxDualCircleBoundaryLine retains all isolated contacts and closed spans
// of continuous boundary contact in the original (unnormalized) line units.
// Interval endpoints are not duplicated in Hits; a support that exists only
// at a junction is retained on the incident interval endpoint.
type BoxDualCircleBoundaryLine struct {
	Hits      []BoxDualCircleLineHit
	Intervals []BoxDualCircleLineInterval
}

// IntersectBoxDualCircleBoundaryRational classifies the original supports of
// a box minus two circular cuts from its lower Z cap to stopZ. The centers
// lie on the lower Y side and each circle strictly clears the other three
// side planes. The tolerance bounds root brackets, never material membership.
// At most eleven distinct polynomial roots and two range endpoints require
// exact ordering; errors and exhaustion publish no partial classification.
func IntersectBoxDualCircleBoundaryRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, stopZ *big.Rat,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryLine, error) {
	return intersectBoxDualCircleBoundaryRational(ctx, min, max, xs, radii, [2]*big.Rat{min[1], min[1]}, stopZ, origin, direction, minT, maxT, o)
}

func intersectBoxDualCircleBoundaryRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY [2]*big.Rat, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryLine, error) {
	var empty BoxDualCircleBoundaryLine
	if ctx == nil || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || minT == nil || maxT == nil || stopZ == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	offset, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: offset.MaxOperations, maxBits: offset.MaxCoefficientBits}
	limits := ExactCurvePlaneEventOptions{MaxOperations: offset.MaxOperations, MaxCoefficientBits: offset.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	for _, v := range []*big.Rat{min[0], min[1], min[2], max[0], max[1], max[2], xs[0], xs[1], radii[0], radii[1],
		origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], centerY[0], centerY[1], stopZ, minT, maxT} {
		if v == nil {
			return empty, ErrInvalid
		}
		if e := b.check(v); e != nil {
			return empty, e
		}
	}
	if minT.Cmp(maxT) > 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return empty, ErrInvalid
	}
	for i := range min {
		if min[i].Cmp(max[i]) >= 0 {
			return empty, ErrDomain
		}
	}
	if min[2].Cmp(stopZ) >= 0 || stopZ.Cmp(max[2]) > 0 {
		return empty, ErrDomain
	}
	if centerY[0].Cmp(min[1]) > 0 || centerY[1].Cmp(min[1]) > 0 {
		return empty, ErrDomain
	}
	for i, r := range radii {
		if r.Sign() <= 0 || new(big.Rat).Sub(xs[i], r).Cmp(min[0]) <= 0 || new(big.Rat).Add(xs[i], r).Cmp(max[0]) >= 0 ||
			new(big.Rat).Add(centerY[i], r).Cmp(max[1]) >= 0 || new(big.Rat).Add(centerY[i], r).Cmp(min[1]) <= 0 {
			return empty, ErrDomain
		}
	}
	// Keep the same transverse overlapping domain as the BRep query. In
	// particular, opposite tangent circle normals cannot hide a union seam.
	dx, dy := new(big.Rat).Sub(xs[0], xs[1]), new(big.Rat).Sub(centerY[0], centerY[1])
	distance2 := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	sum, diff := new(big.Rat).Add(radii[0], radii[1]), new(big.Rat).Sub(radii[0], radii[1])
	sum.Mul(sum, sum)
	diff.Mul(diff, diff)
	for _, q := range []*big.Rat{distance2, sum, diff} {
		if e := b.check(q); e != nil {
			return empty, e
		}
	}
	if distance2.Cmp(sum) >= 0 || distance2.Cmp(diff) <= 0 {
		return empty, ErrDomain
	}
	polynomials := make([]rationalPolynomial, 9)
	for axis := range min {
		polynomials[2*axis] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(min[axis], origin[axis]), new(big.Rat).Neg(direction[axis])})
		polynomials[2*axis+1] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(origin[axis], max[axis]), direction[axis]})
	}
	polynomials[8] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(origin[2], stopZ), direction[2]})
	for i, x := range xs {
		sq := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Mul(radii[i], radii[i])), new(big.Rat), new(big.Rat)}
		for axis, center := range [2]*big.Rat{x, centerY[i]} {
			term := rationalPolynomial{new(big.Rat).Sub(origin[axis], center), direction[axis]}
			v, e := polynomialProduct(term, term, b)
			if e != nil {
				return empty, e
			}
			for j, x := range v {
				sq[j].Add(sq[j], x)
			}
		}
		polynomials[6+i] = trimPolynomial(sq)
	}
	for _, p := range polynomials {
		for _, v := range p {
			if e := b.check(v); e != nil {
				return empty, e
			}
		}
	}
	through := stopZ.Cmp(max[2]) == 0
	line, e := intersectMaterialBoundaryRational(ctx, polynomials, minT, maxT, o, b, limits, func(s []int) (BorePointLocation, BoreBoundaryMask) {
		var support BoxDualCircleSupport
		for axis := range min {
			lo, hi := s[2*axis], s[2*axis+1]
			if lo > 0 || hi > 0 {
				return BoreOutside, 0
			}
			if lo == 0 || hi == 0 {
				support |= BoxDualCircleStockX << axis
			}
		}
		if s[8] <= 0 {
			u := s[6]
			if s[7] < u {
				u = s[7]
			}
			if u < 0 && (through || s[8] < 0) {
				return BoreOutside, 0
			}
			if u == 0 {
				if s[6] == 0 {
					support |= BoxDualCircleFirstWall
				}
				if s[7] == 0 {
					support |= BoxDualCircleSecondWall
				}
			}
			if !through && s[8] == 0 && u <= 0 {
				support |= BoxDualCircleBlindFloor
			}
		}
		if support == 0 {
			return BoreInside, 0
		}
		return BoreBoundary, BoreBoundaryMask(support)
	})
	if e != nil {
		return empty, e
	}
	var out BoxDualCircleBoundaryLine
	for _, h := range line.Hits {
		out.Hits = append(out.Hits, BoxDualCircleLineHit{h.Parameter, h.Bounds, BoxDualCircleSupport(h.Boundary), h.Multiplicity})
	}
	for _, i := range line.Intervals {
		out.Intervals = append(out.Intervals, BoxDualCircleLineInterval{i.Start, i.End, i.StartBounds, i.EndBounds,
			BoxDualCircleSupport(i.Boundary), BoxDualCircleSupport(i.StartBoundary), BoxDualCircleSupport(i.EndBoundary)})
	}
	return out, nil
}
