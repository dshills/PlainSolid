//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativeNestedPrismCutTransition proves the complete planar layers for a strict
// interior axial Cut with genuinely crossing profiles. Stock is original owner
// 0 and tool is owner 1. Both floor regions are Common; their actual model levels
// are Levels()[1] and Levels()[2], with outward normals +Axis and -Axis.
type NativeNestedPrismCutTransition struct {
	source *NativeNestedPrismFuseTransition
	volume *big.Rat
	work   int
}

func (s *NativeNestedPrismCutTransition) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.source.OriginalOwner(i)
}
func (s *NativeNestedPrismCutTransition) Axis() int {
	if s == nil {
		return -1
	}
	return s.source.axis
}
func (s *NativeNestedPrismCutTransition) Levels() [4]*big.Rat {
	if s == nil {
		return [4]*big.Rat{}
	}
	return s.source.Levels()
}
func (s *NativeNestedPrismCutTransition) RetainedProfile() *intersect.AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.source.proofs[0].Region(0)
}
func (s *NativeNestedPrismCutTransition) FloorProfile() *intersect.AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.source.proofs[0].Region(1)
}
func (s *NativeNestedPrismCutTransition) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativeNestedPrismCutTransition) Operations() (int, int) {
	if s == nil {
		return 0, 0
	}
	return s.work, s.source.arrangementWork
}

// ImprintNestedCrossingNativePrismCut reuses the complete original profile
// partition proof, requires stock to be the strict outer axial owner, and proves
// stock volume minus the whole Common middle slab. No artificial source caps or
// changed source intervals are introduced. Tangency/equal cap cases reject.
func ImprintNestedCrossingNativePrismCut(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativeNestedPrismCutTransition, error) {
	return imprintNestedPrismCutMode(ctx, a, b, o, false)
}
func imprintNestedPrismCutMode(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions, sideOpening bool) (*NativeNestedPrismCutTransition, error) {
	source, e := imprintNestedPrismProfilesExtended(ctx, a, b, o, 0, false, sideOpening)
	if e != nil {
		return nil, e
	}
	maximum, bits := o.Native.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits, work: source.work}
	areas := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for r, q := range source.proofs[0].regions {
		for _, c := range q.Cycles() {
			areas[r] = budget.op('+', areas[r], c.SignedAreaA)
		}
	}
	stock := budget.op('*', budget.op('+', areas[0], areas[1]), budget.op('-', source.levels[3], source.levels[0]))
	removed := budget.op('*', areas[1], budget.op('-', source.levels[2], source.levels[1]))
	volume := budget.op('-', stock, removed)
	if budget.err != nil {
		return nil, budget.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if volume.Sign() <= 0 {
		return nil, geom.ErrInvalid
	}
	return &NativeNestedPrismCutTransition{source: source, volume: volume, work: budget.work}, nil
}
