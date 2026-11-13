package thread

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// Engagement proves a single neighboring shaft ridge against its
// corresponding finite nut groove. Both support the returned phase window;
// min gaps are axial and exact at the supplied radius. It is not a global
// interference proof for neighboring runouts or for other radii.
type Engagement struct {
	Complete, Present, WholeWindow bool
	Phase                          [2]*big.Rat
	Ridge                          geom.HelicalPairedWindow
	NutWindow                      geom.HelicalPairedWindow
	WitnessPhase                   *big.Rat
	WitnessShaft, WitnessNut       [2]*big.Rat
	LowerGap, UpperGap             *big.Rat
}

// VerifyEngagement verifies the retained ridge after cutter leftIndex and
// nut cutter leftIndex. Both are identified in the ordered start/turn lattice.
// As a conservative finite gate, the next nut cutter must also be active;
// this can shorten the reported window. All coordinates are relative to the
// common frame and phase follows HelicalProfileSurface's handed convention.
func (p Pair) VerifyEngagement(ctx context.Context, radius *big.Rat, leftIndex int, phase [2]*big.Rat, o geom.SurfacePatchEvaluationOptions) (Engagement, error) {
	var empty Engagement
	if ctx == nil || radius == nil || phase[0] == nil || phase[1] == nil || p.radialRange[0] == nil ||
		radius.Cmp(p.radialRange[0]) < 0 || radius.Cmp(p.radialRange[1]) > 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	shaft, e := p.shaftStarts.CertifyPairedWindow(ctx, radius, leftIndex, phase, o)
	if e != nil {
		return empty, e
	}
	if !shaft.Present {
		return Engagement{Complete: true}, nil
	}
	o.MaxOperations -= shaft.ExactOperations
	if o.MaxOperations < 1 {
		return empty, geom.ErrConvergence
	}
	nut, e := p.nutStarts.CertifyPairedWindow(ctx, radius, leftIndex, shaft.Phase, o)
	if e != nil {
		return empty, e
	}
	if !nut.Present {
		return Engagement{Complete: true}, nil
	}
	o.MaxOperations -= nut.ExactOperations
	if o.MaxOperations < 1 {
		return empty, geom.ErrConvergence
	}
	shaft, e = p.shaftStarts.CertifyPairedWindow(ctx, radius, leftIndex, nut.Phase, o)
	if e != nil {
		return empty, e
	}
	n := nut.WitnessCutters[0]
	a := shaft.WitnessGap
	lower := new(big.Rat).Sub(a[0], n[0])
	upper := new(big.Rat).Sub(n[1], a[1])
	if lower.Sign() <= 0 || upper.Sign() <= 0 {
		return empty, geom.ErrDomain
	}
	// The native construction separately proves each complete cutter is
	// within its own stock. The shared engagement must also be within both.
	lo := new(big.Rat).Mul(exact(p.lead), new(big.Rat).Sub(nut.Phase[0], nut.WitnessPhase))
	hi := new(big.Rat).Mul(exact(p.lead), new(big.Rat).Sub(nut.Phase[1], nut.WitnessPhase))
	if new(big.Rat).Add(n[0], lo).Cmp(p.stockOverlap[0]) <= 0 ||
		new(big.Rat).Add(n[1], hi).Cmp(p.stockOverlap[1]) >= 0 {
		return empty, geom.ErrDomain
	}
	return Engagement{
		Complete: true, Present: true, WholeWindow: shaft.Phase[0].Cmp(phase[0]) == 0 && shaft.Phase[1].Cmp(phase[1]) == 0,
		Phase: shaft.Phase, Ridge: shaft, NutWindow: nut, WitnessPhase: nut.WitnessPhase,
		WitnessShaft: a, WitnessNut: n, LowerGap: lower, UpperGap: upper,
	}, nil
}
