//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ImprintSideOpeningNativePrismCut proves a contained positive shared-side
// footprint with a single original cutter floor strictly inside stock. Existing
// transition native sewing retains every original stock boundary atom.
func ImprintSideOpeningNativePrismCut(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativePrismCutTransition, error) {
	return imprintCrossingPrismCutMode(ctx, a, b, o, true)
}

func certifyNativePrismSideOpening(common *intersect.AffineGeneralPolygonArrangement, tool nativePrismProfile, budget *nativePrismAffineBudget) error {
	if common == nil || len(common.SharedBoundarySpans()) == 0 || len(common.IsolatedCommonPoints()) != 0 {
		return geom.ErrDomain
	}
	side := -1
	for _, s := range common.SharedBoundarySpans() {
		if len(s.Sources) != 2 || s.IntersectionSides[0] == s.IntersectionSides[1] {
			return geom.ErrDomain
		}
		a, b := s.Sources[0], s.Sources[1]
		if a.Owner == 1 {
			a, b = b, a
		}
		if a.Owner != 0 || b.Owner != 1 || a.Ring != 0 || a.Parameter[1].Cmp(a.Parameter[0])*b.Parameter[1].Cmp(b.Parameter[0]) != 1 {
			return geom.ErrDomain
		}
		if side >= 0 && side != a.Edge {
			return geom.ErrDomain
		}
		side = a.Edge
	}
	area := new(big.Rat)
	for _, r := range append([][][2]*big.Rat{tool.trim.Outer}, tool.trim.Holes...) {
		for i, v := range r {
			w := r[(i+1)%len(r)]
			area = budget.op('+', area, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
		}
	}
	area = budget.op('/', area, big.NewRat(2, 1))
	ca := new(big.Rat)
	for _, c := range common.Area().Cycles() {
		ca = budget.op('+', ca, c.SignedAreaA)
	}
	if budget.err != nil {
		return budget.err
	}
	if area.Sign() <= 0 || area.Cmp(ca) != 0 {
		return geom.ErrDomain
	}
	return nil
}

// ImprintInteriorSideOpeningNativePrismCut proves both original tool floors
// strictly inside stock with the same complete one-side footprint certificate.
func ImprintInteriorSideOpeningNativePrismCut(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativeNestedPrismCutTransition, error) {
	return imprintNestedPrismCutMode(ctx, a, b, o, true)
}
