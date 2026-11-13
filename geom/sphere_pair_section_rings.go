package geom

import (
	"context"
	"math/big"
)

// SphereSectionRing is a complete original support circle, oriented with
// retained material on the left viewed from +Z. A cavity ring is clockwise.
type SphereSectionRing struct {
	Source        BoreBoundaryMask
	Center        [2]*big.Rat
	RadiusSquared *big.Rat
	CCW           bool
}
type SphereSectionRingRegion int

const (
	SphereSectionEmpty SphereSectionRingRegion = iota
	SphereSectionDisk
	SphereSectionDisjointDisks
	SphereSectionAnnulus
)

// SpherePairSectionRings describes complete section material in the strictly
// disjoint/nested positive-disk family. Empty is a proved result, not failure.
// Crossing arcs have a distinct HorizontalBoundaryCycle descriptor.
type SpherePairSectionRings struct {
	Height *big.Rat
	Region SphereSectionRingRegion
	Rings  []SphereSectionRing
}

// HorizontalBoundaryRings proves disjointness or strict nesting by squared
// rational inequalities without square roots. Tangent, coincident, zero-radius,
// empty-support and crossing layouts return ErrDomain, never a partial region.
// Section extraction and this fixed predicate stage each use the given limits.
func (s SpherePairMaterial) HorizontalBoundaryRings(ctx context.Context, height float64, o SurfacePatchEvaluationOptions) (SpherePairSectionRings, error) {
	var zero SpherePairSectionRings
	section, e := s.HorizontalSection(ctx, height, o)
	if e != nil {
		return zero, e
	}
	a, b := section.Supports[0], section.Supports[1]
	if a.RadiusSquared.Sign() <= 0 || b.RadiusSquared.Sign() <= 0 {
		return zero, ErrDomain
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	d2 := new(big.Rat)
	for j := 0; j < 2; j++ {
		d := new(big.Rat).Sub(a.Center[j], b.Center[j])
		d2.Add(d2, new(big.Rat).Mul(d, d))
	}
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	sep := new(big.Rat).Sub(new(big.Rat).Sub(d2, a.RadiusSquared), b.RadiusSquared)
	sepRHS := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(a.RadiusSquared, b.RadiusSquared))
	disjoint := sep.Sign() > 0 && square(sep).Cmp(sepRHS) > 0
	large, small, inner := a.RadiusSquared, b.RadiusSquared, 1
	if large.Cmp(small) < 0 {
		large, small, inner = small, large, 0
	}
	gap := new(big.Rat).Sub(new(big.Rat).Sub(large, small), d2)
	gapRHS := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(d2, small))
	nested := gap.Sign() > 0 && square(gap).Cmp(gapRHS) > 0
	for _, v := range []*big.Rat{d2, sep, sepRHS, square(sep), gap, gapRHS, square(gap)} {
		if e := budget.check(v); e != nil {
			return zero, e
		}
	}
	if !disjoint && !nested {
		return zero, ErrDomain
	}
	out := SpherePairSectionRings{Height: new(big.Rat).Set(section.Height), Region: SphereSectionEmpty}
	for i, source := range section.Supports {
		signs := []int{1, 1}
		signs[i] = 0
		if nested && i == inner {
			signs[1-i] = -1
		}
		location, mask := s.location(signs)
		if location != BoreBoundary {
			continue
		}
		if mask != source.Source {
			return zero, ErrConvergence
		}
		out.Rings = append(out.Rings, SphereSectionRing{Source: source.Source, Center: [2]*big.Rat{new(big.Rat).Set(source.Center[0]), new(big.Rat).Set(source.Center[1])}, RadiusSquared: new(big.Rat).Set(source.RadiusSquared), CCW: !(s.op == SphereSetDifference && i == 1)})
	}
	switch len(out.Rings) {
	case 0:
	case 1:
		if !out.Rings[0].CCW {
			return zero, ErrConvergence
		}
		out.Region = SphereSectionDisk
	case 2:
		if disjoint {
			out.Region = SphereSectionDisjointDisks
		} else {
			out.Region = SphereSectionAnnulus
			if !out.Rings[0].CCW || out.Rings[1].CCW {
				return zero, ErrConvergence
			}
		}
	default:
		return zero, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
