package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// finiteParallelTransverseShoulderUnion qualifies one coincident physical cap
// and unequal axial spans. The cap is tiled by two exclusive crescents and a
// shared lens; both original operands own the two shared terminal lens fans.
func finiteParallelTransverseShoulderUnion(ctx context.Context, a, b geom.RawCylinder, tol float64) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	unit, sa := coordinateCylinderAxis(a.Axis)
	other, sb := coordinateCylinderAxis(b.Axis)
	if sa == 0 || sb == 0 || unit.Cross(other) != (geom.Vec3{}) {
		return zero, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	span := func(c geom.RawCylinder, axis geom.Vec3) [2]*big.Rat {
		base, d := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Min)))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Max)))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		return [2]*big.Rat{lo, hi}
	}
	as, bs := span(a, unit), span(b, unit)
	commonLow, commonHigh := as[0].Cmp(bs[0]) == 0, as[1].Cmp(bs[1]) == 0
	if commonLow == commonHigh {
		return zero, false, nil
	}
	tall, short := a, b
	tallOwner := 0
	if new(big.Rat).Sub(as[1], as[0]).Cmp(new(big.Rat).Sub(bs[1], bs[0])) < 0 {
		tall, short, tallOwner = b, a, 1
	}
	unit, _ = coordinateCylinderAxis(tall.Axis)
	ts, ss := span(tall, unit), span(short, unit)
	lower := ts[0].Cmp(ss[0]) == 0
	d2 := new(big.Rat)
	for k, v := range []float64{unit.X, unit.Y, unit.Z} {
		if v == 0 {
			d := new(big.Rat).Sub(q([]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}[k]), q([]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}[k]))
			d2.Add(d2, new(big.Rat).Mul(d, d))
		}
	}
	sum := new(big.Rat).Add(q(a.Radius), q(b.Radius))
	diff := new(big.Rat).Sub(q(a.Radius), q(b.Radius))
	if d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0 {
		return zero, false, nil
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tol))
	station := ss[1]
	if !lower {
		station = ss[0]
	}
	if new(big.Rat).Sub(station, ts[0]).Cmp(pad) <= 0 || new(big.Rat).Sub(ts[1], station).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	// Original physical source caps must survive exactly before any recharting.
	for _, c := range []geom.RawCylinder{tall, short} {
		for _, t := range []float64{c.Min, c.Max} {
			for k, x := range []float64{c.Origin.X, c.Origin.Y, c.Origin.Z} {
				if _, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(x), new(big.Rat).Mul(q([]float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[k]), q(t)))); e != nil {
					return zero, true, e
				}
			}
		}
	}
	h, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ts[1], ts[0]))
	if e != nil {
		return zero, true, e
	}
	z, e := geom.RepresentedExactScalar(new(big.Rat).Sub(station, ts[0]))
	if e != nil {
		return zero, true, e
	}
	full := short
	full.Axis = unit
	full.Min = 0
	full.Max = h
	for k, v := range []float64{unit.X, unit.Y, unit.Z} {
		if v != 0 {
			x, e := geom.RepresentedExactScalar(new(big.Rat).Mul(ts[0], q(v)))
			if e != nil {
				return zero, true, e
			}
			*[3]*float64{&full.Origin.X, &full.Origin.Y, &full.Origin.Z}[k] = x
		}
	}
	da, ok, e := finiteParallelOverlap(ctx, tall, full, tol, Difference)
	if e != nil || !ok {
		return zero, ok, e
	}
	db, ok, e := finiteParallelOverlap(ctx, full, tall, tol, Difference)
	if e != nil {
		return zero, true, e
	}
	if !ok {
		return zero, true, geom.ErrDomain
	}
	lens, ok, e := finiteParallelOverlap(ctx, tall, full, tol, Intersection)
	if e != nil {
		return zero, true, e
	}
	if !ok {
		return zero, true, geom.ErrDomain
	}
	d, f, l := da.Faces(), db.Faces(), lens.Faces()
	if len(d) != 4 || len(f) != 4 || len(l) != 6 {
		return zero, true, geom.ErrDomain
	}
	outerLow, outerHigh := d[0], d[0]
	outerLow.V = Interval{0, z}
	outerHigh.V = Interval{z, h}
	shortWall, innerWall := f[0], l[0]
	shortWall.V = Interval{0, z}
	innerWall.V = Interval{z, h}
	commonSide, endSide := 0, 1
	commonCap, endCap := 2, 3
	commonLens, endLens := [2]int{1, 4}, [2]int{2, 5}
	if !lower {
		shortWall.V = Interval{z, h}
		innerWall.V = Interval{0, z}
		commonSide, endSide = 1, 0
		commonCap, endCap = 3, 2
		commonLens, endLens = [2]int{2, 5}, [2]int{1, 4}
	}
	faces := []Face{outerLow, outerHigh, shortWall, innerWall, d[commonCap], f[commonCap], l[commonLens[0]], l[commonLens[1]]}
	shoulder := f[endCap]
	ruled, ok := shoulder.Surface.(ruledArcs)
	if !ok {
		return zero, true, geom.ErrDomain
	}
	ruled.height = z
	shoulder.Surface = ruled
	faces = append(faces, shoulder, d[endCap], l[endLens[0]], l[endLens[1]])
	solid, e := joinPrimitivePatches(faces, tol)
	if e != nil {
		return zero, true, e
	}
	if e = ctx.Err(); e != nil {
		return zero, true, e
	}
	out := FiniteCylinderPairUnionResult{Solid: solid}
	out.WallFaces[tallOwner] = []int{0, 1, 3}
	out.WallFaces[1-tallOwner] = []int{2}
	add := func(owner, side int, ids ...int) {
		source := a
		if owner == 1 {
			source = b
		}
		if source.Axis.Dot(unit) < 0 {
			side = 1 - side
		}
		if side == 0 {
			out.LowerCapFaces[owner] = append(out.LowerCapFaces[owner], ids...)
		} else {
			out.UpperCapFaces[owner] = append(out.UpperCapFaces[owner], ids...)
		}
	}
	add(tallOwner, commonSide, 4, 6, 7)
	add(1-tallOwner, commonSide, 5, 6, 7)
	add(tallOwner, endSide, 9, 10, 11)
	add(1-tallOwner, endSide, 8)
	return out, true, nil
}
