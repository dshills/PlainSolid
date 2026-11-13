package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// finiteCoaxialEqualCylinderUnion joins overlapping or end-to-end stock
// intervals on the same coordinate axis. The full coincident end is interior
// to the resulting solid and only exposed endpoint caps have ancestry.
func finiteCoaxialEqualCylinderUnion(ctx context.Context, a, b geom.RawCylinder) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	axis, scale := coordinateCylinderAxis(a.Axis)
	_, otherScale := coordinateCylinderAxis(b.Axis)
	if scale == 0 || otherScale == 0 || a.Radius != b.Radius {
		return zero, false, nil
	}
	for i, dir := range [3]float64{axis.X, axis.Y, axis.Z} {
		if dir == 0 && [3]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}[i] != [3]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}[i] {
			return zero, false, nil
		}
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	var endpoints [2][2]*big.Rat
	var spans [2][2]*big.Rat
	for i, c := range [2]geom.RawCylinder{a, b} {
		origin, speed := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		for j, x := range [2]float64{c.Min, c.Max} {
			endpoints[i][j] = new(big.Rat).Add(origin, new(big.Rat).Mul(q(x), speed))
		}
		spans[i] = endpoints[i]
		if speed.Sign() < 0 {
			spans[i][0], spans[i][1] = spans[i][1], spans[i][0]
		}
	}
	minRat := func(x, y *big.Rat) *big.Rat {
		if x.Cmp(y) <= 0 {
			return x
		}
		return y
	}
	maxRat := func(x, y *big.Rat) *big.Rat {
		if x.Cmp(y) >= 0 {
			return x
		}
		return y
	}
	common := new(big.Rat).Sub(minRat(spans[0][1], spans[1][1]), maxRat(spans[0][0], spans[1][0]))
	if common.Sign() < 0 {
		return zero, false, nil
	}
	lo, hi := minRat(spans[0][0], spans[1][0]), maxRat(spans[0][1], spans[1][1])
	start := a.Origin
	for i, dir := range [3]float64{axis.X, axis.Y, axis.Z} {
		if dir == 0 {
			continue
		}
		v, err := geom.RepresentedExactScalar(new(big.Rat).Mul(lo, q(dir)))
		if err != nil {
			return zero, true, err
		}
		if _, err = geom.RepresentedExactScalar(new(big.Rat).Mul(hi, q(dir))); err != nil {
			return zero, true, err
		}
		*[3]*float64{&start.X, &start.Y, &start.Z}[i] = v
	}
	h, err := geom.RepresentedExactScalar(new(big.Rat).Sub(hi, lo))
	if err != nil {
		return zero, true, err
	}
	frame, err := geom.FrameFromNormal(start, axis)
	if err != nil {
		return zero, true, err
	}
	s, err := Cylinder(frame, a.Radius, h)
	if err != nil {
		return zero, true, err
	}
	if err = ctx.Err(); err != nil {
		return zero, true, err
	}
	u := FiniteCylinderPairUnionResult{Solid: s, WallFaces: [2][]int{{0}, {0}}}
	if common.Sign() == 0 {
		u.CommonVolumeSquared = common
	} else {
		u.CommonVolumePiCoefficient = common.Mul(common, new(big.Rat).Mul(q(a.Radius), q(a.Radius)))
	}
	for i, pair := range endpoints {
		for j, e := range pair {
			cap := 0
			if e.Cmp(lo) == 0 {
				cap = 1
			} else if e.Cmp(hi) == 0 {
				cap = 2
			}
			if cap != 0 && j == 0 {
				u.LowerCapFaces[i] = []int{cap}
			} else if cap != 0 {
				u.UpperCapFaces[i] = []int{cap}
			}
		}
	}
	return u, true, nil
}
