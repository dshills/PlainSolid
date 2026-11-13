package geom

import (
	"context"
	"math/big"
)

// SphereSectionArc is a directed exact arc of Center/RadiusSquared, viewed from
// +Z. Its open interior satisfies InteriorSign*(Covector dot XY+Offset)>0.
// This half-plane plus the circle uniquely selects the intended arc without
// atan2 or rounded radii. Endpoints index the cycle's exact Contacts array.
// CCW=false reverses the circle orientation (the cutter arc in Difference).
type SphereSectionArc struct {
	Source                   BoreBoundaryMask
	Center                   [2]*big.Rat
	RadiusSquared            *big.Rat
	StartContact, EndContact int
	CCW                      bool
	Covector                 [2]*big.Rat
	Offset                   *big.Rat
	InteriorSign             int
}

// SpherePairSectionCycle is one complete simple boundary cycle for two strictly
// crossing positive-radius section disks. Arcs are source ordered (A then B),
// endpoint-connected, and orient retained section material to their left.
// This API deliberately excludes tangent/coincident/pole/noncrossing layouts.
// All coefficients and contacts are caller-owned; endpoints remain radicals.
type SpherePairSectionCycle struct {
	Height   *big.Rat
	Contacts []SphereSectionContact
	Arcs     [2]SphereSectionArc
}

// HorizontalBoundaryCycle proves the two directed arcs from exact circle
// contacts and radical-axis signs. It returns ErrDomain for every layout
// outside the strictly crossing positive-disk family, with no partial cycle.
// Section construction and the fixed coefficient stage each use the supplied
// work/bit limits; no unbounded refinement or numeric angle inference occurs.
func (s SpherePairMaterial) HorizontalBoundaryCycle(ctx context.Context, height float64, o SurfacePatchEvaluationOptions) (SpherePairSectionCycle, error) {
	var zero SpherePairSectionCycle
	section, e := s.HorizontalSection(ctx, height, o)
	if e != nil {
		return zero, e
	}
	if section.CoincidentCircle || section.Supports[0].RadiusSquared.Sign() <= 0 || section.Supports[1].RadiusSquared.Sign() <= 0 || len(section.Contacts) != 2 {
		return zero, ErrDomain
	}
	for i, p := range section.Contacts {
		if p.Sign != 2*i-1 || p.Radicand.Sign() <= 0 || p.Location != BoreBoundary || p.Boundary != SpherePairFirst|SpherePairSecond {
			return zero, ErrDomain
		}
	}
	// The section constructor orders contacts as foot -/+ sqrt(rad)*perp(B-A).
	// Consequently A's inside-B CCW arc is 0->1, and B's inside-A is 1->0.
	out := SpherePairSectionCycle{Height: new(big.Rat).Set(section.Height), Contacts: section.Contacts}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	for owner := 0; owner < 2; owner++ {
		source := section.Supports[owner]
		other := section.Supports[1-owner]
		arc := SphereSectionArc{Source: source.Source, RadiusSquared: new(big.Rat).Set(source.RadiusSquared), CCW: true, Offset: new(big.Rat)}
		inside := s.op == SphereSetIntersection || (s.op == SphereSetDifference && owner == 1)
		arc.InteriorSign = -1
		if inside {
			arc.InteriorSign = 1
		}
		arc.StartContact, arc.EndContact = 1, 0
		if owner == 1 {
			arc.StartContact, arc.EndContact = 0, 1
		}
		if inside {
			arc.StartContact, arc.EndContact = arc.EndContact, arc.StartContact
		}
		if s.op == SphereSetDifference && owner == 1 {
			arc.CCW = false
			arc.StartContact, arc.EndContact = arc.EndContact, arc.StartContact
		}
		for j := 0; j < 2; j++ {
			arc.Center[j] = new(big.Rat).Set(source.Center[j])
			arc.Covector[j] = new(big.Rat).Sub(other.Center[j], source.Center[j])
			arc.Offset.Sub(arc.Offset, new(big.Rat).Mul(arc.Covector[j], section.Contacts[0].Base[j]))
		}
		for _, v := range []*big.Rat{arc.Covector[0], arc.Covector[1], arc.Offset} {
			if e := budget.check(v); e != nil {
				return zero, e
			}
		}
		out.Arcs[owner] = arc
	}
	if out.Arcs[0].EndContact != out.Arcs[1].StartContact || out.Arcs[1].EndContact != out.Arcs[0].StartContact {
		return zero, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
