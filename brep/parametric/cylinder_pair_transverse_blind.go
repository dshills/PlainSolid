package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// finiteParallelTransverseBlind restores the disk lens below (or above) an
// exact original cutter cap, or on both sides of an interior cutter interval.
// The footprint is a proved transverse two-circle
// arrangement; analytic arc evaluation/sewing retains the usual tolerance.
func finiteParallelTransverseBlind(ctx context.Context, a, b geom.RawCylinder, tol float64) (FiniteCylinderPairDifferenceResult, bool, error) {
	var zero FiniteCylinderPairDifferenceResult
	axis, speed := coordinateCylinderAxis(a.Axis)
	other, bs := coordinateCylinderAxis(b.Axis)
	if speed == 0 || bs == 0 || axis.Cross(other) != (geom.Vec3{}) {
		return zero, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	span := func(c geom.RawCylinder) [2]*big.Rat {
		base, d := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Min)))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Max)))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		return [2]*big.Rat{lo, hi}
	}
	as, cs := span(a), span(b)
	lower := cs[0].Cmp(as[0]) <= 0 && cs[1].Cmp(as[0]) > 0 && cs[1].Cmp(as[1]) < 0
	upper := cs[1].Cmp(as[1]) >= 0 && cs[0].Cmp(as[0]) > 0 && cs[0].Cmp(as[1]) < 0
	interior := cs[0].Cmp(as[0]) > 0 && cs[1].Cmp(as[1]) < 0
	if !lower && !upper && !interior {
		return zero, false, nil
	}
	// Only strict transverse disk overlap belongs to this constructor.
	d2 := new(big.Rat)
	for k, v := range []float64{axis.X, axis.Y, axis.Z} {
		if v == 0 {
			d := new(big.Rat).Sub(q([]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}[k]), q([]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}[k]))
			d2.Add(d2, new(big.Rat).Mul(d, d))
		}
	}
	sum := new(big.Rat).Add(q(a.Radius), q(b.Radius))
	dif := new(big.Rat).Sub(q(a.Radius), q(b.Radius))
	if d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(dif, dif)) <= 0 {
		return zero, false, nil
	}
	floor := cs[0]
	if lower {
		floor = cs[1]
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tol))
	if new(big.Rat).Sub(floor, as[0]).Cmp(pad) <= 0 || new(big.Rat).Sub(as[1], floor).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	if interior && (new(big.Rat).Sub(cs[1], cs[0]).Cmp(pad) <= 0 || new(big.Rat).Sub(as[1], cs[1]).Cmp(pad) <= 0) {
		return zero, true, geom.ErrDomain
	}
	h, e := geom.RepresentedExactScalar(new(big.Rat).Sub(as[1], as[0]))
	if e != nil {
		return zero, true, e
	}
	z, e := geom.RepresentedExactScalar(new(big.Rat).Sub(floor, as[0]))
	if e != nil {
		return zero, true, e
	}
	for _, c := range []geom.RawCylinder{a, b} {
		for _, t := range []float64{c.Min, c.Max} {
			for k, x := range []float64{c.Origin.X, c.Origin.Y, c.Origin.Z} {
				if _, e = geom.RepresentedExactScalar(new(big.Rat).Add(q(x), new(big.Rat).Mul(q([]float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[k]), q(t)))); e != nil {
					return zero, true, e
				}
			}
		}
	}
	clipped := b
	clipped.Axis = axis
	clipped.Min = 0
	clipped.Max = h
	for k, v := range []float64{axis.X, axis.Y, axis.Z} {
		if v != 0 {
			x, e := geom.RepresentedExactScalar(new(big.Rat).Mul(as[0], q(v)))
			if e != nil {
				return zero, true, e
			}
			*[3]*float64{&clipped.Origin.X, &clipped.Origin.Y, &clipped.Origin.Z}[k] = x
		}
	}
	crescent, ok, e := finiteParallelOverlap(ctx, a, clipped, tol, Difference)
	if e != nil {
		return zero, true, e
	}
	if !ok {
		return zero, true, geom.ErrDomain
	}
	lens, ok, e := finiteParallelOverlap(ctx, a, clipped, tol, Intersection)
	if e != nil {
		return zero, true, e
	}
	if !ok {
		return zero, true, geom.ErrDomain
	}
	d, l := crescent.Faces(), lens.Faces()
	if len(d) != 4 || len(l) != 6 {
		return zero, true, geom.ErrDomain
	}
	if interior {
		zhi, err := geom.RepresentedExactScalar(new(big.Rat).Sub(cs[1], as[0]))
		if err != nil {
			return zero, true, err
		}
		out, err := assembleTransverseInteriorNotch(ctx, d, l, axis, h, [2]float64{z, zhi}, cs, b.Axis.Dot(axis) > 0, tol)
		return out, true, err
	}
	low, high := d[0], d[0]
	low.V = Interval{0, z}
	high.V = Interval{z, h}
	cut := d[1]
	restore := l[0]
	cut.V = Interval{z, h}
	restore.V = Interval{0, z}
	floorIDs, endIDs := [2]int{2, 5}, [2]int{1, 4}
	if lower {
		cut.V = Interval{0, z}
		restore.V = Interval{z, h}
		floorIDs, endIDs = [2]int{1, 4}, [2]int{2, 5}
	}
	faces := []Face{low, high, cut, d[2], d[3], restore}
	for _, id := range floorIDs {
		f := l[id]
		fan, ok := f.Surface.(arcFan)
		if !ok {
			return zero, true, geom.ErrDomain
		}
		// Set the original physical cutter plane directly. Subtracting the
		// previous fan height could round an otherwise representable floor.
		for k, v := range []float64{axis.X, axis.Y, axis.Z} {
			if v != 0 {
				value, err := geom.RepresentedExactScalar(new(big.Rat).Mul(floor, q(v)))
				if err != nil {
					return zero, true, err
				}
				*[3]*float64{&fan.center.X, &fan.center.Y, &fan.center.Z}[k] = value
			}
		}
		fan.height = z
		f.Surface = fan
		faces = append(faces, f)
	}
	for _, id := range endIDs {
		faces = append(faces, l[id])
	}
	solid, e := joinPrimitivePatches(faces, tol)
	if e != nil {
		return zero, true, e
	}
	if e = ctx.Err(); e != nil {
		return zero, true, e
	}
	return FiniteCylinderPairDifferenceResult{Solids: [2]Solid{solid, {}}, StockWallFaces: [2][]int{{0, 1, 5}}, CutterWallFaces: [2][]int{{2}}, StockCapFaces: [2][]int{{3, 4, 8, 9}}, CutterCapFaces: [2][]int{{6, 7}}}, true, nil
}
