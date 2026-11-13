package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// finiteParallelCylinderAxialDifference removes a complete stock cross
// section, proved with exact transverse disk containment. Each retained raw
// interval preserves the stock chart direction and the original cutter cap
// face attribution, including antiparallel input directions.
func finiteParallelCylinderAxialDifference(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, bool, error) {
	var zero FiniteCylinderPairDifferenceResult
	axis, speed := coordinateCylinderAxis(stock.Axis)
	_, cutterSpeed := coordinateCylinderAxis(cutter.Axis)
	if speed == 0 || cutterSpeed == 0 || stock.Radius > cutter.Radius {
		return zero, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	distance2 := new(big.Rat)
	av, bv := [3]float64{stock.Origin.X, stock.Origin.Y, stock.Origin.Z}, [3]float64{cutter.Origin.X, cutter.Origin.Y, cutter.Origin.Z}
	for i, z := range [3]float64{axis.X, axis.Y, axis.Z} {
		if z == 0 {
			d := new(big.Rat).Sub(q(av[i]), q(bv[i]))
			distance2.Add(distance2, new(big.Rat).Mul(d, d))
		}
	}
	clearance := new(big.Rat).Sub(q(cutter.Radius), q(stock.Radius))
	if distance2.Cmp(new(big.Rat).Mul(clearance, clearance)) > 0 {
		return zero, false, nil
	}
	base := q(stock.Origin.Dot(axis))
	cutterBase, cutterScale := q(cutter.Origin.Dot(axis)), q(cutter.Axis.Dot(axis))
	// Work in stock raw parameter units without rounding physical cap values.
	var bounds [2]*big.Rat
	owners := [2]int{0, 1}
	for i, t := range [2]float64{cutter.Min, cutter.Max} {
		x := new(big.Rat).Add(cutterBase, new(big.Rat).Mul(cutterScale, q(t)))
		bounds[i] = x.Quo(x.Sub(x, base), q(speed))
	}
	if bounds[0].Cmp(bounds[1]) > 0 {
		bounds[0], bounds[1] = bounds[1], bounds[0]
		owners[0], owners[1] = owners[1], owners[0]
	}
	lo, hi := q(stock.Min), q(stock.Max)
	if bounds[1].Cmp(lo) == 0 || bounds[0].Cmp(hi) == 0 {
		s, err := nativeCoordinateCylinder(stock)
		if err != nil {
			return zero, true, err
		}
		if err := ctx.Err(); err != nil {
			return zero, true, err
		}
		return FiniteCylinderPairDifferenceResult{Solids: [2]Solid{s, {}}, StockWallFaces: [2][]int{{0}}, StockCapFaces: [2][]int{{1, 2}}}, true, nil
	}
	if bounds[1].Cmp(lo) < 0 || bounds[0].Cmp(hi) > 0 {
		return zero, false, nil
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	width := func(l, h *big.Rat) *big.Rat { return new(big.Rat).Mul(new(big.Rat).Sub(h, l), q(speed)) }
	removedLo, removedHi := bounds[0], bounds[1]
	if removedLo.Cmp(lo) < 0 {
		removedLo = lo
	}
	if removedHi.Cmp(hi) > 0 {
		removedHi = hi
	}
	if width(removedLo, removedHi).Cmp(pad) <= 0 || q(stock.Radius).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	var out FiniteCylinderPairDifferenceResult
	n := 0
	for i, t := range [2][2]*big.Rat{{lo, removedLo}, {removedHi, hi}} {
		if t[0].Cmp(t[1]) == 0 {
			continue
		}
		if width(t[0], t[1]).Cmp(pad) <= 0 {
			return zero, true, geom.ErrDomain
		}
		// Construct directly in model axial coordinates: a physical cut cap
		// may be representable even when its parameter on the raw axis isn't.
		start := stock.Origin
		for k, z := range [3]float64{axis.X, axis.Y, axis.Z} {
			if z != 0 {
				for j, p := range t {
					x := new(big.Rat).Add(base, new(big.Rat).Mul(q(speed), p))
					v, err := geom.RepresentedExactScalar(x.Mul(x, q(z)))
					if err != nil {
						return zero, true, err
					}
					if j == 0 {
						*[3]*float64{&start.X, &start.Y, &start.Z}[k] = v
					}
				}
			}
		}
		h, err := geom.RepresentedExactScalar(width(t[0], t[1]))
		if err != nil {
			return zero, true, err
		}
		frame, err := geom.FrameFromNormal(start, axis)
		if err != nil {
			return zero, true, err
		}
		solid, err := Cylinder(frame, stock.Radius, h)
		if err != nil {
			return zero, true, err
		}
		out.Solids[n], out.StockWallFaces[n] = solid, []int{0}
		out.StockCapFaces[n] = []int{1 + i}
		out.CutterCapFaces[n] = []int{2 - i}
		out.AxialCutCaps[owners[i]] = &FiniteCylinderCutCap{Solid: n, Face: 2 - i}
		n++
	}
	if n == 2 {
		gap := width(removedLo, removedHi)
		out.MinimumGapSquared = new(big.Rat).Mul(gap, gap)
	}
	if err := ctx.Err(); err != nil {
		return zero, true, err
	}
	return out, true, nil
}
