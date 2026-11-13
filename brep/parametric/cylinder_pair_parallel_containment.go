package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// parallelCoordinateContainment proves full containment in both the axial
// interval and transverse disk. Contacts of the cap planes or the disk
// boundary are retained as equality, never mistaken for positive clearance.
// If the two closed cylinders are identical both flags are true.
func parallelCoordinateContainment(a, b geom.RawCylinder) (contained [2]bool) {
	axis, sa := coordinateCylinderAxis(a.Axis)
	_, sb := coordinateCylinderAxis(b.Axis)
	if sa == 0 || sb == 0 {
		return contained
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var spans [2][2]*big.Rat
	for i, c := range [2]geom.RawCylinder{a, b} {
		base, scale := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(q(c.Min), scale))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(q(c.Max), scale))
		if scale.Sign() < 0 {
			lo, hi = hi, lo
		}
		spans[i] = [2]*big.Rat{lo, hi}
	}
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	dist2 := new(big.Rat)
	av, bv := [3]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}, [3]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}
	for i, z := range [3]float64{axis.X, axis.Y, axis.Z} {
		if z == 0 {
			dist2.Add(dist2, sq(new(big.Rat).Sub(q(av[i]), q(bv[i]))))
		}
	}
	radii := [2]float64{a.Radius, b.Radius}
	for i := range 2 {
		r := new(big.Rat).Sub(q(radii[1-i]), q(radii[i]))
		contained[i] = r.Sign() >= 0 && dist2.Cmp(sq(r)) <= 0 && spans[i][0].Cmp(spans[1-i][0]) >= 0 && spans[i][1].Cmp(spans[1-i][1]) <= 0
	}
	return contained
}

func unionContainedParallelCylinder(ctx context.Context, a, b geom.RawCylinder, contained [2]bool) (FiniteCylinderPairUnionResult, error) {
	var zero FiniteCylinderPairUnionResult
	// If both operands are identical, use A's native surface orientation.
	outer := 0
	if contained[0] && !contained[1] {
		outer = 1
	}
	operands := [2]geom.RawCylinder{a, b}
	s, err := nativeCoordinateCylinder(operands[outer])
	if err != nil {
		return zero, err
	}
	out := FiniteCylinderPairUnionResult{Solid: s, CommonVolumePiCoefficient: nativeCoordinateCylinderVolumePi(operands[1-outer])}
	out.WallFaces[outer], out.LowerCapFaces[outer], out.UpperCapFaces[outer] = []int{0}, []int{1}, []int{2}
	if contained[0] && contained[1] {
		out.WallFaces[1] = []int{0}
		if a.Axis.Dot(b.Axis) < 0 {
			out.LowerCapFaces[1], out.UpperCapFaces[1] = []int{2}, []int{1}
		} else {
			out.LowerCapFaces[1], out.UpperCapFaces[1] = []int{1}, []int{2}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}

func nativeCoordinateCylinderVolumePi(c geom.RawCylinder) *big.Rat {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	_, scale := coordinateCylinderAxis(c.Axis)
	r := q(c.Radius)
	return new(big.Rat).Mul(new(big.Rat).Mul(r, r), new(big.Rat).Mul(q(scale), new(big.Rat).Sub(q(c.Max), q(c.Min))))
}
