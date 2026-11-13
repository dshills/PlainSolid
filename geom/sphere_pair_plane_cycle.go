package geom

import (
	"context"
	"math/big"
)

// SpherePlaneSectionArc is a directed arc in a Gram chart. Unlike a Euclidean
// UV circle, its equation is (uv-Center)^T Gram (uv-Center)=RadiusSquared.
// Its open interior satisfies InteriorSign*(Covector dot uv+Offset)>0.
// CCW is counterclockwise in the returned UV chart, not about the raw normal.
// Endpoint indices refer to Contacts, whose order is induced by Normal cross
// the projected center displacement. Each coefficient is caller-owned.
type SpherePlaneSectionArc struct {
	Source                   BoreBoundaryMask
	Center                   [2]*big.Rat
	RadiusSquared            *big.Rat
	StartContact, EndContact int
	CCW                      bool
	Covector                 [2]*big.Rat
	Offset                   *big.Rat
	InteriorSign             int
}

// SpherePairPlaneCycle describes one simple material-left cycle in the returned
// UV chart, with source-ordered arcs. ChartNormalSign is the sign of
// (Basis[0] cross Basis[1]) dot Normal. No solid assembly is asserted.
type SpherePairPlaneCycle struct {
	Chart           SpherePairPlaneSupports
	Contacts        []SpherePlaneSectionContact
	Arcs            [2]SpherePlaneSectionArc
	ChartNormalSign int
}

// PlaneBoundaryCycle accepts only two strictly crossing positive support
// circles whose common points are exposed in the original regularized set.
// Tangent, coincident, pole and noncrossing cases return ErrDomain atomically.
// Section construction and this fixed coefficient stage each use the supplied
// bounds; directions and selectors are derived exactly, without numeric angles.
func (s SpherePairMaterial) PlaneBoundaryCycle(ctx context.Context, origin, normal Vec3, o SurfacePatchEvaluationOptions) (SpherePairPlaneCycle, error) {
	var zero SpherePairPlaneCycle
	section, e := s.PlaneSection(ctx, origin, normal, o)
	if e != nil {
		return zero, e
	}
	if section.CoincidentCircle || section.Chart.Supports[0].Sign <= 0 || section.Chart.Supports[1].Sign <= 0 || len(section.Contacts) != 2 {
		return zero, ErrDomain
	}
	for i, p := range section.Contacts {
		if p.Sign != 2*i-1 || p.Radicand.Sign() <= 0 || p.Location != BoreBoundary || p.Boundary != SpherePairFirst|SpherePairSecond {
			return zero, ErrDomain
		}
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	orientation := new(big.Rat)
	for j := 0; j < 3; j++ {
		a, b := (j+1)%3, (j+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(section.Chart.Basis[0][a], section.Chart.Basis[1][b]), new(big.Rat).Mul(section.Chart.Basis[0][b], section.Chart.Basis[1][a]))
		orientation.Add(orientation, new(big.Rat).Mul(cross, section.Chart.Normal[j]))
	}
	if e := budget.check(orientation); e != nil {
		return zero, e
	}
	if orientation.Sign() == 0 {
		return zero, ErrConvergence
	}
	out := SpherePairPlaneCycle{Chart: section.Chart, Contacts: section.Contacts, ChartNormalSign: orientation.Sign()}
	for owner := 0; owner < 2; owner++ {
		source, other := section.Chart.Supports[owner], section.Chart.Supports[1-owner]
		arc := SpherePlaneSectionArc{Source: source.Source, RadiusSquared: new(big.Rat).Set(source.RadiusSquared), CCW: true, Offset: new(big.Rat), InteriorSign: -1, StartContact: 1, EndContact: 0}
		inside := s.op == SphereSetIntersection || (s.op == SphereSetDifference && owner == 1)
		if owner == 1 {
			arc.StartContact, arc.EndContact = 0, 1
		}
		if inside {
			arc.InteriorSign = 1
			arc.StartContact, arc.EndContact = arc.EndContact, arc.StartContact
		}
		if s.op == SphereSetDifference && owner == 1 {
			arc.CCW = false
			arc.StartContact, arc.EndContact = arc.EndContact, arc.StartContact
		}
		if orientation.Sign() < 0 {
			arc.StartContact, arc.EndContact = arc.EndContact, arc.StartContact
		}
		// Project the physical radical-axis covector into chart coordinates.
		for j := 0; j < 2; j++ {
			arc.Center[j] = new(big.Rat).Set(source.Center[j])
			arc.Covector[j] = new(big.Rat)
			for k := 0; k < 3; k++ {
				d := new(big.Rat).Sub(other.WorldCenter[k], source.WorldCenter[k])
				arc.Covector[j].Add(arc.Covector[j], new(big.Rat).Mul(d, section.Chart.Basis[j][k]))
			}
			arc.Offset.Sub(arc.Offset, new(big.Rat).Mul(arc.Covector[j], section.Contacts[0].UVBase[j]))
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
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
