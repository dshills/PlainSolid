package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// finiteParallelOverlap requires identical physical cap planes and a strictly
// transverse circular intersection. Raw endpoints and local center differences
// must be exactly representable; circle trim angles use the existing analytic
// CombineParallelCylinders implementation and its tolerance checks.
func finiteParallelOverlap(ctx context.Context, a, b geom.RawCylinder, tol float64, op BooleanOp) (Solid, bool, error) {
	axis, sa := coordinateCylinderAxis(a.Axis)
	other, sb := coordinateCylinderAxis(b.Axis)
	if sa == 0 || sb == 0 || axis.Cross(other) != (geom.Vec3{}) {
		return Solid{}, false, nil
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	ends := func(c geom.RawCylinder) [2]*big.Rat {
		base, delta := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		return [2]*big.Rat{new(big.Rat).Add(base, new(big.Rat).Mul(delta, q(c.Min))), new(big.Rat).Add(base, new(big.Rat).Mul(delta, q(c.Max)))}
	}
	ae, be := ends(a), ends(b)
	if be[0].Cmp(be[1]) > 0 {
		be[0], be[1] = be[1], be[0]
	}
	if ae[0].Cmp(be[0]) != 0 || ae[1].Cmp(be[1]) != 0 {
		return Solid{}, false, nil
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tol))
	d2 := new(big.Rat)
	av, bv := [3]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}, [3]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}
	for i, v := range [3]float64{axis.X, axis.Y, axis.Z} {
		if v == 0 {
			d := new(big.Rat).Sub(q(bv[i]), q(av[i]))
			d2.Add(d2, new(big.Rat).Mul(d, d))
		}
	}
	// This branch never takes containment, separation, or radial tangency.
	sum := new(big.Rat).Add(q(a.Radius), q(b.Radius))
	diff := new(big.Rat).Abs(new(big.Rat).Sub(q(a.Radius), q(b.Radius)))
	if d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0 {
		return Solid{}, false, nil
	}
	sum.Sub(sum, pad)
	diff.Add(diff, pad)
	hr := new(big.Rat).Sub(ae[1], ae[0])
	if sum.Sign() <= 0 || d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0 || hr.Cmp(pad) <= 0 || q(a.Radius).Cmp(pad) <= 0 || q(b.Radius).Cmp(pad) <= 0 {
		return Solid{}, true, geom.ErrDomain
	}
	h, err := geom.RepresentedExactScalar(hr)
	if err != nil {
		return Solid{}, true, err
	}
	var start geom.Vec3
	for i, p := range [3]*float64{&start.X, &start.Y, &start.Z} {
		delta := q([3]float64{a.Axis.X, a.Axis.Y, a.Axis.Z}[i])
		*p, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(av[i]), new(big.Rat).Mul(delta, q(a.Min))))
		if err != nil {
			return Solid{}, true, err
		}
		if _, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(av[i]), new(big.Rat).Mul(delta, q(a.Max)))); err != nil {
			return Solid{}, true, err
		}
		for _, t := range []float64{b.Min, b.Max} {
			if _, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(bv[i]), new(big.Rat).Mul(q([3]float64{b.Axis.X, b.Axis.Y, b.Axis.Z}[i]), q(t)))); err != nil {
				return Solid{}, true, err
			}
		}
	}
	frame, err := geom.FrameFromNormal(start, axis)
	if err != nil {
		return Solid{}, true, err
	}
	var center geom.Vec2
	for i, dir := range [2]geom.Vec3{frame.XAxis(), frame.YAxis()} {
		v, e := geom.RepresentedExactScalar(new(big.Rat).Sub(q(b.Origin.Dot(dir)), q(a.Origin.Dot(dir))))
		if e != nil {
			return Solid{}, true, e
		}
		if i == 0 {
			center.X = v
		} else {
			center.Y = v
		}
	}
	// Signed coordinate frame projection must round-trip the original transverse
	// coordinates; arbitrary oblique placement is intentionally unsupported.
	p := frame.ToWorld(geom.Vec3{X: center.X, Y: center.Y})
	for i, v := range [3]float64{axis.X, axis.Y, axis.Z} {
		if v == 0 && [3]float64{p.X, p.Y, p.Z}[i] != bv[i] {
			return Solid{}, true, geom.ErrDomain
		}
	}
	s, err := CombineParallelCylinders(frame, DiskProfile{Radius: a.Radius}, DiskProfile{Center: center, Radius: b.Radius}, h, op, tol)
	if err != nil {
		return Solid{}, true, err
	}
	if err = ctx.Err(); err != nil {
		return Solid{}, true, err
	}
	return s, true, nil
}

func finiteParallelOverlapUnion(ctx context.Context, a, b geom.RawCylinder, tol float64) (FiniteCylinderPairUnionResult, bool, error) {
	s, ok, err := finiteParallelOverlap(ctx, a, b, tol, Union)
	if !ok || err != nil {
		return FiniteCylinderPairUnionResult{}, ok, err
	}
	out := FiniteCylinderPairUnionResult{Solid: s, WallFaces: [2][]int{{0}, {3}}, LowerCapFaces: [2][]int{{1}, {4}}, UpperCapFaces: [2][]int{{2}, {5}}}
	if a.Axis.Dot(b.Axis) < 0 {
		out.LowerCapFaces[1], out.UpperCapFaces[1] = out.UpperCapFaces[1], out.LowerCapFaces[1]
	}
	// The common volume generally involves inverse trigonometric terms; neither
	// rational proof field is applicable.
	return out, true, nil
}
func finiteParallelOverlapDifference(ctx context.Context, a, b geom.RawCylinder, tol float64) (FiniteCylinderPairDifferenceResult, bool, error) {
	s, ok, err := finiteParallelOverlap(ctx, a, b, tol, Difference)
	if !ok || err != nil {
		return FiniteCylinderPairDifferenceResult{}, ok, err
	}
	return FiniteCylinderPairDifferenceResult{Solids: [2]Solid{s, {}}, StockWallFaces: [2][]int{{0}}, CutterWallFaces: [2][]int{{1}}, StockCapFaces: [2][]int{{2, 3}}}, true, nil
}
