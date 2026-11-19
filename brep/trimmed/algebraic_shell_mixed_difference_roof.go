//go:build research

package trimmed

import (
	"context"
	"math/big"
)

// MixedRoofMaterialConstraint names the exact whole-cell material witness.
// Inside/above/below margins have model Z units; footprint margins have XY units.
// AboveOtherRoof may certify only the cell intersection with the opposing
// footprint; the remainder is outside that solid by XY coordinates.
type MixedRoofMaterialConstraint uint8

const (
	MixedRoofInsideOther MixedRoofMaterialConstraint = iota
	MixedRoofOutsideXMin
	MixedRoofOutsideXMax
	MixedRoofOutsideYMin
	MixedRoofOutsideYMax
	MixedRoofBelowOtherFloor
	MixedRoofAboveOtherRoof
)

// TessellateDifferenceRoofFragmentCertified covers an original A-B roof.
// It admits hole-free fragments of certified six-face polynomial identity-XY
// graph boxes only; both complete source boxes are independently reproved.
// A roofs are strictly outside B away from explicit trim collars. B roofs are
// inside A and published with reversed orientation. It does not alter the
// intersection-only or strict-interior entry points.
func (n *CertifiedMixedAlgebraicShellBoundary) TessellateDifferenceRoofFragmentCertified(ctx context.Context, face, oppositeRoof int, o MixedRoofCollarOptions) (*CertifiedMixedRoofFragment, error) {
	return n.tessellateRoofFragment(ctx, face, oppositeRoof, o, true, false)
}

func mixedRoofMaterialClearance(host, other *mixedRoofBox, xy [3][2]*big.Rat, outside bool) (*big.Rat, MixedRoofMaterialConstraint, error) {
	return mixedRoofMaterialClearanceMode(host, other, xy, outside, false)
}
func mixedRoofMaterialClearanceMode(host, other *mixedRoofBox, xy [3][2]*big.Rat, outside, clipped bool) (*big.Rat, MixedRoofMaterialConstraint, error) {
	if !outside {
		m, e := mixedRoofTriangleClearance(host, other, xy)
		return m, MixedRoofInsideOther, e
	}
	zero := new(big.Rat)
	// The source itself must be in its original chart and above its floor.
	for _, p := range xy {
		if p[0].Cmp(ratInset(host.domain.UMin)) <= 0 || p[0].Cmp(ratInset(host.domain.UMax)) >= 0 || p[1].Cmp(ratInset(host.domain.VMin)) <= 0 || p[1].Cmp(ratInset(host.domain.VMax)) >= 0 {
			return zero, MixedRoofInsideOther, nil
		}
	}
	a, e := mixedRoofTrianglePower(host.roof, xy)
	if e != nil {
		return nil, 0, e
	}
	lo, hi := mixedRoofPolynomialRange(a)
	source := new(big.Rat).Sub(lo, host.floor)
	if source.Sign() <= 0 {
		return zero, 0, nil
	}
	d := other.domain
	bounds := []float64{d.UMin, d.UMax, d.VMin, d.VMax}
	for k, b := range bounds {
		var margin *big.Rat
		for _, p := range xy {
			v := new(big.Rat).Sub(p[k/2], ratInset(b))
			if k%2 == 0 {
				v.Neg(v)
			}
			if margin == nil || v.Cmp(margin) < 0 {
				margin = v
			}
		}
		if margin.Sign() > 0 {
			return margin, MixedRoofMaterialConstraint(int(MixedRoofOutsideXMin) + k), nil
		}
	}
	below := new(big.Rat).Sub(other.floor, hi)
	if below.Sign() > 0 {
		return below, MixedRoofBelowOtherFloor, nil
	}
	if clipped {
		return mixedRoofClippedAbove(host, other, xy)
	}
	// Only evaluate the opposing graph on its actual chart domain.
	for _, p := range xy {
		if p[0].Cmp(ratInset(d.UMin)) < 0 || p[0].Cmp(ratInset(d.UMax)) > 0 || p[1].Cmp(ratInset(d.VMin)) < 0 || p[1].Cmp(ratInset(d.VMax)) > 0 {
			return zero, 0, nil
		}
	}
	b, e := mixedRoofTrianglePower(other.roof, xy)
	if e != nil {
		return nil, 0, e
	}
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			a[i][j].Sub(a[i][j], b[i][j])
		}
	}
	gap, _ := mixedRoofPolynomialRange(a)
	if gap.Sign() > 0 {
		return gap, MixedRoofAboveOtherRoof, nil
	}
	return zero, 0, nil
}
