package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// finiteParallelTransverseStaggeredUnion uses three strictly positive axial
// bands. Only the exclusive crescents survive on the two internal source caps.
func finiteParallelTransverseStaggeredUnion(ctx context.Context, a, b geom.RawCylinder, tol float64) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	unit, sa := coordinateCylinderAxis(a.Axis)
	other, sb := coordinateCylinderAxis(b.Axis)
	if sa == 0 || sb == 0 || unit.Cross(other) != (geom.Vec3{}) {
		return zero, false, nil
	}
	// A positive coordinate axis makes the ordering independent of raw handedness.
	unit = geom.Vec3{X: math.Abs(unit.X), Y: math.Abs(unit.Y), Z: math.Abs(unit.Z)}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	span := func(c geom.RawCylinder) [2]*big.Rat {
		base, d := q(c.Origin.Dot(unit)), q(c.Axis.Dot(unit))
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Min)))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(d, q(c.Max)))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		return [2]*big.Rat{lo, hi}
	}
	as, bs := span(a), span(b)
	first, last, owner := a, b, 0
	if as[0].Cmp(bs[0]) > 0 {
		as, bs = bs, as
		first, last, owner = b, a, 1
	}
	if !(as[0].Cmp(bs[0]) < 0 && bs[0].Cmp(as[1]) < 0 && as[1].Cmp(bs[1]) < 0) {
		return zero, false, nil
	}
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
	stations := [4]*big.Rat{as[0], bs[0], as[1], bs[1]}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tol))
	for i := 0; i < 3; i++ {
		if new(big.Rat).Sub(stations[i+1], stations[i]).Cmp(pad) <= 0 {
			return zero, true, geom.ErrDomain
		}
	}
	for _, c := range []geom.RawCylinder{a, b} {
		for _, t := range []float64{c.Min, c.Max} {
			for k, x := range []float64{c.Origin.X, c.Origin.Y, c.Origin.Z} {
				if _, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(x), new(big.Rat).Mul(q([]float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[k]), q(t)))); e != nil {
					return zero, true, e
				}
			}
		}
	}
	var z [4]float64
	for i := 1; i < 4; i++ {
		v, e := geom.RepresentedExactScalar(new(big.Rat).Sub(stations[i], stations[0]))
		if e != nil {
			return zero, true, e
		}
		z[i] = v
	}
	full := func(c geom.RawCylinder) (geom.RawCylinder, error) {
		c.Axis = unit
		c.Min = 0
		c.Max = z[3]
		for k, v := range []float64{unit.X, unit.Y, unit.Z} {
			if v != 0 {
				x, e := geom.RepresentedExactScalar(stations[0])
				if e != nil {
					return c, e
				}
				*[3]*float64{&c.Origin.X, &c.Origin.Y, &c.Origin.Z}[k] = x
			}
		}
		return c, nil
	}
	fa, e := full(first)
	if e != nil {
		return zero, true, e
	}
	fb, e := full(last)
	if e != nil {
		return zero, true, e
	}
	da, ok, e := finiteParallelOverlap(ctx, fa, fb, tol, Difference)
	if e != nil || !ok {
		return zero, ok, e
	}
	db, ok, e := finiteParallelOverlap(ctx, fb, fa, tol, Difference)
	if e != nil {
		return zero, true, e
	}
	if !ok {
		return zero, true, geom.ErrDomain
	}
	lens, ok, e := finiteParallelOverlap(ctx, fa, fb, tol, Intersection)
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
	wall := func(face Face, lo, hi float64) Face { face.V = Interval{lo, hi}; return face }
	faces := []Face{wall(d[0], z[0], z[1]), wall(d[0], z[1], z[2]), wall(f[0], z[1], z[2]), wall(f[0], z[2], z[3]), wall(l[0], z[0], z[1]), wall(l[3], z[2], z[3]), d[2], l[1], l[4], f[3], l[2], l[5]}
	for _, x := range []struct {
		face Face
		z    float64
	}{{f[2], z[1]}, {d[3], z[2]}} {
		face := x.face
		ruled, ok := face.Surface.(ruledArcs)
		if !ok {
			return zero, true, geom.ErrDomain
		}
		ruled.height = x.z
		face.Surface = ruled
		faces = append(faces, face)
	}
	solid, e := joinPrimitivePatches(faces, tol)
	if e != nil {
		return zero, true, e
	}
	if e = ctx.Err(); e != nil {
		return zero, true, e
	}
	out := FiniteCylinderPairUnionResult{Solid: solid}
	out.WallFaces[owner] = []int{0, 1, 4}
	out.WallFaces[1-owner] = []int{2, 3, 5}
	add := func(o, side int, ids ...int) {
		c := a
		if o == 1 {
			c = b
		}
		if c.Axis.Dot(unit) < 0 {
			side = 1 - side
		}
		if side == 0 {
			out.LowerCapFaces[o] = append(out.LowerCapFaces[o], ids...)
		} else {
			out.UpperCapFaces[o] = append(out.UpperCapFaces[o], ids...)
		}
	}
	add(owner, 0, 6, 7, 8)
	add(owner, 1, 13)
	add(1-owner, 0, 12)
	add(1-owner, 1, 9, 10, 11)
	return out, true, nil
}
