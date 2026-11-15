package geom

import "math/big"

// proveLineEdgeRoot proves existence on an original-domain edge. Combined with
// a strict contraction bound in the 3D cell, this proves its sole contact even
// when strict Krawczyk interior inclusion cannot hold at the boundary.
func proveLineEdgeRoot(p *SurfacePatch, origin, direction [3]*big.Rat, original ParameterRectangle, lo, hi float64, b *polynomialBudget) (bool, error) {
	axis := 0
	for i := 1; i < 3; i++ {
		if new(big.Rat).Abs(direction[i]).Cmp(new(big.Rat).Abs(direction[axis])) > 0 {
			axis = i
		}
	}
	dk, ok := direction[axis], origin[axis]
	start, end := new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)
	domain := p.Domain()
	for edge := SurfaceEdge(0); edge < 4; edge++ {
		outer := edge == SurfaceEdgeUMin && domain.UMin == original.UMin || edge == SurfaceEdgeUMax && domain.UMax == original.UMax || edge == SurfaceEdgeVMin && domain.VMin == original.VMin || edge == SurfaceEdgeVMax && domain.VMax == original.VMax
		if !outer {
			continue
		}
		indices := p.edgeIndices(edge)
		net := make([]exactHomogeneous, len(indices))
		for i, index := range indices {
			net[i] = p.net[index]
		}
		var polynomials []rationalPolynomial
		for coordinateAxis := 0; coordinateAxis < 3; coordinateAxis++ {
			if coordinateAxis == axis {
				continue
			}
			coeff := make([]*big.Rat, len(net))
			di, oi := direction[coordinateAxis], origin[coordinateAxis]
			for i, h := range net {
				first := new(big.Rat).Sub(h[coordinateAxis], new(big.Rat).Mul(oi, h[3]))
				second := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(ok, h[3]))
				coeff[i] = new(big.Rat).Sub(new(big.Rat).Mul(dk, first), new(big.Rat).Mul(di, second))
				if e := b.check(coeff[i]); e != nil {
					return false, e
				}
			}
			poly, e := bernsteinToPower(coeff, b)
			if e != nil {
				return false, e
			}
			if len(poly) > 0 {
				polynomials = append(polynomials, poly)
			}
		}
		if len(polynomials) == 0 {
			continue
		} // Edge coincidence is deliberately unresolved.
		gcd := polynomials[0]
		if len(polynomials) > 1 {
			var e error
			gcd, e = polynomialGCD(gcd, polynomials[1], b)
			if e != nil {
				return false, e
			}
		}
		if len(gcd) < 2 {
			continue
		}
		exists := false
		_, e := isolatePolynomialRoots(gcd, big.NewRat(0, 1), big.NewRat(1, 1), b, 4096, func(a, c *big.Rat, _ int) (bool, error) {
			restricted, e := restrictExactBezier(net, a, c, b)
			if e != nil {
				return false, e
			}
			var lower, upper *big.Rat
			for _, h := range restricted {
				value := new(big.Rat).Quo(new(big.Rat).Sub(h[axis], new(big.Rat).Mul(ok, h[3])), new(big.Rat).Mul(dk, h[3]))
				if e := b.check(value); e != nil {
					return false, e
				}
				if lower == nil || value.Cmp(lower) < 0 {
					lower = new(big.Rat).Set(value)
				}
				if upper == nil || value.Cmp(upper) > 0 {
					upper = new(big.Rat).Set(value)
				}
			}
			if upper.Cmp(start) < 0 || lower.Cmp(end) > 0 {
				return true, nil
			}
			if lower.Cmp(start) >= 0 && upper.Cmp(end) <= 0 {
				exists = true
				return true, nil
			}
			return false, nil
		})
		if e != nil {
			return false, e
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}
