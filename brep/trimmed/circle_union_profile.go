package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

type unionDisk struct {
	center geom.Vec2
	radius float64
	frame  geom.Frame
}
type unionArc struct {
	disk, start, end int
	from, to         float64
}

// circleRelation compares the exact represented squared center distance with
// the squared sum/difference of represented radii, before constructing angles.
func circleRelation(a, b unionDisk, sum bool) int {
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dx := new(big.Rat).Sub(rat(a.center.X), rat(b.center.X))
	dy := new(big.Rat).Sub(rat(a.center.Y), rat(b.center.Y))
	d2 := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	r := new(big.Rat).Sub(rat(a.radius), rat(b.radius))
	if sum {
		r.Add(rat(a.radius), rat(b.radius))
	}
	return d2.Cmp(new(big.Rat).Mul(r, r))
}

func unionDotSign(a, b geom.Vec3) int {
	x, y := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}
	sum := new(big.Rat)
	for i := range x {
		sum.Add(sum, new(big.Rat).Mul(new(big.Rat).SetFloat64(x[i]), new(big.Rat).SetFloat64(y[i])))
	}
	return sum.Sign()
}
