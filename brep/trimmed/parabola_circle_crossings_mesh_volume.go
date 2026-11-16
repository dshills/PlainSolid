package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// The two nodes are the free-coordinate diameter extrema. Each signed disk
// region is a horizontal half disk plus or minus the polynomial strip
// between its center ordinate and the original parabola. Integrate these
// separately, retaining rational and pi contributions until the final bound.
func parabolaCircleCrossingMeshVolumes(height [2]parabolaCircleMeshHeight,
	p *geom.SplineGraphParabolaCircleCrossings) [4][2]*big.Rat {
	free := p.FreeAxis
	center := [2]*big.Rat{p.CircleCenter[free], p.CircleCenter[1-free]}
	stripPowers := parabolaCircleMeshStripPowers(p.ParabolaControls, free)
	a, b := p.Nodes[0].Parameters[free], p.Nodes[1].Parameters[free]
	var spans [28]*big.Rat
	pa, pb := big.NewRat(1, 1), big.NewRat(1, 1)
	for i := 1; i < len(spans); i++ {
		pa.Mul(pa, a)
		pb.Mul(pb, b)
		spans[i] = new(big.Rat).Sub(pb, pa)
	}
	var centerPowers [10]*big.Rat
	centerPowers[0] = big.NewRat(1, 1)
	for i := 1; i < len(centerPowers); i++ {
		centerPowers[i] = new(big.Rat).Mul(centerPowers[i-1], center[1])
	}
	signedLowerStrip, signedUnit := new(big.Rat), new(big.Rat)
	signedDiskHalf, signedDiskBelowCorrection := new(big.Rat), new(big.Rat)
	circle := geom.GraphContactCircle{Center: center, RadiusSquared: p.RadiusSquared}
	for owner, h := range height {
		for u, row := range h.coefficients {
			for v, c := range row {
				if c == nil {
					continue
				}
				m, n := u, v
				if free == 1 {
					m, n = v, u
				}
				coefficient := new(big.Rat).Set(c)
				if owner == 1 {
					coefficient.Neg(coefficient)
				}
				signedUnit.Add(signedUnit, new(big.Rat).Quo(coefficient, big.NewRat(int64((m+1)*(n+1)), 1)))
				localStrip := new(big.Rat)
				for k, term := range stripPowers[n+1] {
					integral := new(big.Rat).Quo(term, big.NewRat(int64((n+1)*(m+k+1)), 1))
					signedLowerStrip.Add(signedLowerStrip, new(big.Rat).Mul(coefficient, integral))
					localStrip.Add(localStrip, integral.Mul(integral, spans[m+k+1]))
				}
				centerStrip := new(big.Rat).Mul(centerPowers[n+1], spans[m+1])
				centerStrip.Quo(centerStrip, big.NewRat(int64((n+1)*(m+1)), 1))
				localStrip.Sub(localStrip, centerStrip)
				localStrip.Sub(localStrip, parabolaCircleUpperHalfOddMoment(center, p.CircleRadius, m, n))
				signedDiskBelowCorrection.Add(signedDiskBelowCorrection, localStrip.Mul(localStrip, coefficient))
				disk := parabolaCircleDiskMoment(circle, m, n)
				signedDiskHalf.Add(signedDiskHalf, disk.Mul(disk, coefficient))
			}
		}
	}
	signedDiskHalf.Quo(signedDiskHalf, big.NewRat(2, 1))
	pi := parabolaCircleMeshPi()
	bound := func(rational, factor *big.Rat) (out [2]*big.Rat) {
		for i, q := range pi {
			out[i] = new(big.Rat).Add(rational, new(big.Rat).Mul(q, factor))
		}
		if factor.Sign() < 0 {
			out[0], out[1] = out[1], out[0]
		}
		return out
	}
	negativeHalf := new(big.Rat).Neg(signedDiskHalf)
	above := new(big.Rat).Sub(signedUnit, signedLowerStrip)
	return [4][2]*big.Rat{
		bound(new(big.Rat).Sub(signedLowerStrip, signedDiskBelowCorrection), negativeHalf),
		bound(above.Add(above, signedDiskBelowCorrection), negativeHalf),
		bound(signedDiskBelowCorrection, signedDiskHalf),
		bound(new(big.Rat).Neg(signedDiskBelowCorrection), signedDiskHalf),
	}
}

func parabolaCircleUpperHalfOddMoment(center [2]*big.Rat, radius *big.Rat, u, v int) *big.Rat {
	// Centered even dependent powers contribute half the full disk moment.
	// Centered odd dependent powers change sign across the diameter. On the
	// upper half disk they integrate vertically to (r²-x²)^k / (2k).
	power := func(x *big.Rat, n int) *big.Rat {
		r := big.NewRat(1, 1)
		for range n {
			r.Mul(r, x)
		}
		return r
	}
	moment := new(big.Rat)
	for i := 0; i <= u; i += 2 {
		for j := 1; j <= v; j += 2 {
			k := (j + 1) / 2
			sum := new(big.Rat)
			for l := 0; l <= k; l++ {
				n := new(big.Int).Binomial(int64(k), int64(l))
				if l%2 != 0 {
					n.Neg(n)
				}
				sum.Add(sum, new(big.Rat).SetFrac(n, big.NewInt(int64(k*(i+2*l+1)))))
			}
			sum.Mul(sum, power(radius, i+j+2))
			sum.Mul(sum, power(center[0], u-i))
			sum.Mul(sum, power(center[1], v-j))
			sum.Mul(sum, new(big.Rat).SetInt(new(big.Int).Binomial(int64(u), int64(i))))
			sum.Mul(sum, new(big.Rat).SetInt(new(big.Int).Binomial(int64(v), int64(j))))
			moment.Add(moment, sum)
		}
	}
	return moment
}
