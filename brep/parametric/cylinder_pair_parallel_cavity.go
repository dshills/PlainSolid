package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// finiteParallelCylinderCavity qualifies a strictly interior cutter. The
// independent inward shell is assembled only after both axial and radial
// distances are proved larger than the requested sewing clearance.
// Operands and tolerance have already been validated by the dispatcher.
func finiteParallelCylinderCavity(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, bool, error) {
	var zero FiniteCylinderPairDifferenceResult
	axis, sa := coordinateCylinderAxis(stock.Axis)
	_, sb := coordinateCylinderAxis(cutter.Axis)
	if sa == 0 || sb == 0 || stock.Radius <= cutter.Radius {
		return zero, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	span := func(c geom.RawCylinder) [2]*big.Rat {
		z, scale := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		lo := new(big.Rat).Add(z, new(big.Rat).Mul(scale, q(c.Min)))
		hi := new(big.Rat).Add(z, new(big.Rat).Mul(scale, q(c.Max)))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		return [2]*big.Rat{lo, hi}
	}
	a, b := span(stock), span(cutter)
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if new(big.Rat).Sub(b[0], a[0]).Cmp(pad) <= 0 || new(big.Rat).Sub(a[1], b[1]).Cmp(pad) <= 0 ||
		new(big.Rat).Sub(b[1], b[0]).Cmp(pad) <= 0 || q(cutter.Radius).Cmp(pad) <= 0 {
		return zero, false, nil
	}
	gap := new(big.Rat).Sub(new(big.Rat).Sub(q(stock.Radius), q(cutter.Radius)), pad)
	if gap.Sign() <= 0 {
		return zero, false, nil
	}
	distanceSquared := new(big.Rat)
	sv, cv := [3]float64{stock.Origin.X, stock.Origin.Y, stock.Origin.Z}, [3]float64{cutter.Origin.X, cutter.Origin.Y, cutter.Origin.Z}
	for i, direction := range [3]float64{axis.X, axis.Y, axis.Z} {
		if direction == 0 {
			d := new(big.Rat).Sub(q(sv[i]), q(cv[i]))
			distanceSquared.Add(distanceSquared, new(big.Rat).Mul(d, d))
		}
	}
	if distanceSquared.Cmp(new(big.Rat).Mul(gap, gap)) >= 0 {
		return zero, false, nil
	}
	outer, err := nativeCoordinateCylinder(stock)
	if err != nil {
		return zero, true, err
	}
	inner, err := nativeCoordinateCylinder(cutter)
	if err != nil {
		return zero, true, err
	}
	for i := range inner.faces {
		inner.faces[i].Reversed = !inner.faces[i].Reversed
	}
	solid, err := joinIndependent(outer, inner)
	if err != nil {
		return zero, true, err
	}
	if err = ctx.Err(); err != nil {
		return zero, true, err
	}
	return FiniteCylinderPairDifferenceResult{
		Solids:          [2]Solid{solid, {}},
		StockWallFaces:  [2][]int{{0}},
		CutterWallFaces: [2][]int{{3}},
		StockCapFaces:   [2][]int{{1, 2}},
		CutterCapFaces:  [2][]int{{4, 5}},
	}, true, nil
}
