package parametric

import (
	"math/big"
	"plainsolid/geom"
)

// clipParallelCoveringCutter changes only the internal construction recipe.
// The original cutter wall is unchanged; the artificial caps are stock caps,
// never reported as retained cutter-cap fragments.
func clipParallelCoveringCutter(stock, cutter geom.RawCylinder) (geom.RawCylinder, bool, error) {
	unit, speed := coordinateCylinderAxis(stock.Axis)
	other, cs := coordinateCylinderAxis(cutter.Axis)
	if speed == 0 || cs == 0 || unit.Cross(other) != (geom.Vec3{}) {
		return geom.RawCylinder{}, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	span := func(c geom.RawCylinder) [2]*big.Rat {
		base, delta := q(c.Origin.Dot(unit)), q(c.Axis.Dot(unit))
		a := new(big.Rat).Add(base, new(big.Rat).Mul(delta, q(c.Min)))
		b := new(big.Rat).Add(base, new(big.Rat).Mul(delta, q(c.Max)))
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		return [2]*big.Rat{a, b}
	}
	a, b := span(stock), span(cutter)
	if b[0].Cmp(a[0]) > 0 || b[1].Cmp(a[1]) < 0 {
		return geom.RawCylinder{}, false, nil
	}
	// Preserve the existing exact represented-source cap domain even though
	// the exterior cutter caps do not appear on the result boundary.
	for _, c := range []geom.RawCylinder{stock, cutter} {
		for _, t := range []float64{c.Min, c.Max} {
			for k, x := range []float64{c.Origin.X, c.Origin.Y, c.Origin.Z} {
				d := []float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[k]
				if _, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(x), new(big.Rat).Mul(q(d), q(t)))); e != nil {
					return geom.RawCylinder{}, true, e
				}
			}
		}
	}
	h, e := geom.RepresentedExactScalar(new(big.Rat).Sub(a[1], a[0]))
	if e != nil {
		return geom.RawCylinder{}, true, e
	}
	origin := cutter.Origin
	for k, v := range []float64{unit.X, unit.Y, unit.Z} {
		if v != 0 {
			x, e := geom.RepresentedExactScalar(new(big.Rat).Mul(a[0], q(v)))
			if e != nil {
				return geom.RawCylinder{}, true, e
			}
			*[3]*float64{&origin.X, &origin.Y, &origin.Z}[k] = x
		}
	}
	return geom.RawCylinder{Origin: origin, Axis: unit, Min: 0, Max: h, Radius: cutter.Radius}, true, nil
}
