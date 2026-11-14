package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// NativePrismTiedCapContribution identifies one triangle of the shared terminal
// cap. Parent count is two on Common and one on owner-only material. Face IDs
// refer to the original input solids; no parent is inferred from plane equality.
type NativePrismTiedCapContribution struct {
	Face    int
	Parents []NativePrismCapParent
}
type NativePrismCapParent struct{ Operand, OriginalFace int }

// NativeTiedPrismFuse retains both original supports of the shared terminal cap.
// Faces with Operand/OriginalFace=-1 and Region=3 have two parents, available in
// TiedCapContributions. Other faces retain the standard single-source history.
type NativeTiedPrismFuse struct {
	native *NativeNestedPrismFuse
	caps   []NativePrismTiedCapContribution
}

func (s *NativeTiedPrismFuse) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.native.Solid()
}
func (s *NativeTiedPrismFuse) Faces() []NativePrismFuseFaceUse {
	if s == nil {
		return nil
	}
	return s.native.Faces()
}
func (s *NativeTiedPrismFuse) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	return s.native.WallUses()
}
func (s *NativeTiedPrismFuse) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return s.native.source.ExactMaterialVolume()
}
func (s *NativeTiedPrismFuse) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.native.source.OriginalOwner(i)
}
func (s *NativeTiedPrismFuse) Operations() (int, int, int) {
	if s == nil {
		return 0, 0, 0
	}
	a, b := s.native.source.Operations()
	return a, b, s.native.work
}
func (s *NativeTiedPrismFuse) TiedCapContributions() []NativePrismTiedCapContribution {
	if s == nil {
		return nil
	}
	out := append([]NativePrismTiedCapContribution(nil), s.caps...)
	for i := range out {
		out[i].Parents = append([]NativePrismCapParent(nil), out[i].Parents...)
	}
	return out
}

// FuseTiedCrossingNativePrisms requires positive axial overlap with precisely
// one tied terminal and genuinely transverse profiles. Exact Common and both
// owner-only partitions are independently certified before native sewing.
func FuseTiedCrossingNativePrisms(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativeTiedPrismFuse, error) {
	return fuseTiedCrossingNativePrisms(ctx, a, b, o, o.Native)
}
func fuseTiedCrossingNativePrisms(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions, publish AffineArrangementPrismOptions) (*NativeTiedPrismFuse, error) {
	p, e := imprintNestedPrismProfilesMode(ctx, a, b, o, -1, true)
	if e != nil {
		return nil, e
	}
	n, e := buildNestedPrismFuseMode(ctx, p, publish, true)
	if e != nil {
		return nil, e
	}
	out := &NativeTiedPrismFuse{native: n}
	low := p.levels[0].Cmp(p.levels[1]) == 0
	parents := [2]NativePrismCapParent{}
	found := [2]bool{}
	for _, w := range n.walls {
		owner := w.Use.Operand
		cap := w.Use.UpperCap
		if low {
			cap = w.Use.LowerCap
		}
		parent := NativePrismCapParent{owner, cap}
		if found[owner] && parents[owner] != parent {
			return nil, geom.ErrInvalid
		}
		parents[owner] = parent
		found[owner] = true
	}
	if !found[0] || !found[1] {
		return nil, geom.ErrInvalid
	}
	for _, f := range n.faces {
		if f.Region == 3 {
			out.caps = append(out.caps, NativePrismTiedCapContribution{f.Face, []NativePrismCapParent{parents[0], parents[1]}})
		} else if f.Operand >= 0 && f.OriginalFace == parents[f.Operand].OriginalFace {
			out.caps = append(out.caps, NativePrismTiedCapContribution{f.Face, []NativePrismCapParent{parents[f.Operand]}})
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
