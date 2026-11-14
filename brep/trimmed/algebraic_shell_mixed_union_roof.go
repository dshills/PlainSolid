package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TessellateUnionRoofFragmentCertified admits hole-bearing roof fragments only
// after complete original-support projected loop and incidence proof. Both
// original sources must qualify as six-face polynomial identity-XY graph boxes.
// Every non-collar cell is outside the opposing material. Shared outer and hole
// boundary samples remain fixed; the result is not by itself a closed shell.
func (n *CertifiedMixedAlgebraicShellBoundary) TessellateUnionRoofFragmentCertified(ctx context.Context, face, oppositeRoof int, o MixedRoofCollarOptions) (*CertifiedMixedRoofFragment, error) {
	return n.tessellateRoofFragment(ctx, face, oppositeRoof, o, false, true)
}

// A cell need not lie in the other chart. Prove the height inequality only on
// its exact intersection with that chart's footprint; the remaining cell is
// outside the opposing solid by its XY coordinates. No polynomial continuation
// is used as an assertion of source geometry outside its original domain.
func mixedRoofClippedAbove(host, other *mixedRoofBox, xy [3][2]*big.Rat) (*big.Rat, MixedRoofMaterialConstraint, error) {
	poly := append([][2]*big.Rat(nil), xy[:]...)
	d := other.domain
	for k, value := range []float64{d.UMin, d.UMax, d.VMin, d.VMax} {
		if len(poly) == 0 {
			return new(big.Rat), MixedRoofAboveOtherRoof, nil
		}
		axis := k / 2
		bound := ratInset(value)
		side := func(p [2]*big.Rat) *big.Rat {
			v := new(big.Rat).Sub(p[axis], bound)
			if k%2 == 1 {
				v.Neg(v)
			}
			return v
		}
		var next [][2]*big.Rat
		previous := poly[len(poly)-1]
		sp := side(previous)
		for _, current := range poly {
			sc := side(current)
			if (sp.Sign() < 0) != (sc.Sign() < 0) {
				t := new(big.Rat).Quo(sp, new(big.Rat).Sub(sp, sc))
				q := [2]*big.Rat{}
				for j := 0; j < 2; j++ {
					q[j] = new(big.Rat).Add(previous[j], new(big.Rat).Mul(t, new(big.Rat).Sub(current[j], previous[j])))
					if q[j].Num().BitLen() > 4096 || q[j].Denom().BitLen() > 4096 {
						return nil, 0, geom.ErrConvergence
					}
				}
				next = append(next, q)
			}
			if sc.Sign() >= 0 {
				next = append(next, current)
			}
			previous, sp = current, sc
		}
		poly = nil
		equal := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
		for _, p := range next {
			if len(poly) == 0 || !equal(poly[len(poly)-1], p) {
				poly = append(poly, p)
			}
		}
		if len(poly) > 1 && equal(poly[0], poly[len(poly)-1]) {
			poly = poly[:len(poly)-1]
		}
		if len(poly) > 8 {
			return nil, 0, geom.ErrConvergence
		}
	}
	var margin *big.Rat
	for j := 1; j+1 < len(poly); j++ {
		tri := [3][2]*big.Rat{poly[0], poly[j], poly[j+1]}
		area := roofUVArea(tri)
		if area.Sign() < 0 {
			return nil, 0, geom.ErrInvalid
		}
		if area.Sign() == 0 {
			continue
		}
		a, e := mixedRoofTrianglePower(host.roof, tri)
		if e != nil {
			return nil, 0, e
		}
		b, e := mixedRoofTrianglePower(other.roof, tri)
		if e != nil {
			return nil, 0, e
		}
		for i := 0; i <= 5; i++ {
			for k := 0; k <= 5-i; k++ {
				a[i][k].Sub(a[i][k], b[i][k])
			}
		}
		lo, _ := mixedRoofPolynomialRange(a)
		if lo.Sign() <= 0 {
			return new(big.Rat), MixedRoofAboveOtherRoof, nil
		}
		if margin == nil || lo.Cmp(margin) < 0 {
			margin = lo
		}
	}
	if margin == nil {
		margin = new(big.Rat)
	}
	return margin, MixedRoofAboveOtherRoof, nil
}
