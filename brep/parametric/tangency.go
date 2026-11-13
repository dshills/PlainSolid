package parametric

import (
	"math/big"

	"plainsolid/geom"
)

// tangent compares represented coordinates and radii before a rounded distance
// equality is treated as exact tangency.
func tangent(a, b geom.Vec3, ra, rb float64, internal bool) bool {
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sum := new(big.Rat)
	for _, p := range [][2]float64{{a.X, b.X}, {a.Y, b.Y}, {a.Z, b.Z}} {
		d := new(big.Rat).Sub(rat(p[0]), rat(p[1]))
		sum.Add(sum, d.Mul(d, d))
	}
	r := rat(ra)
	if internal {
		r.Sub(r, rat(rb))
	} else {
		r.Add(r, rat(rb))
	}
	return sum.Cmp(r.Mul(r, r)) == 0
}
