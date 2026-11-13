package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// finiteObliqueParallelCylinderCommon uses a represented similarity frame
// only after exact arithmetic proves its axes, projected offsets and cap
// locations. The local Boolean preserves native circular chart geometry.
func finiteObliqueParallelCylinderCommon(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	return finiteObliqueParallelCylinderSet(ctx, a, b, tolerance, Intersection)
}

// Union and difference use the same exact similarity only for a transverse
// disk overlap between identical physical cap planes. Unequal cap planes use
// the separately qualified coordinate-axis constructions after lossless
// projection and exact original world-cap validation.
func finiteObliqueParallelCylinderSet(ctx context.Context, a, b geom.RawCylinder, tolerance float64, op BooleanOp) (FiniteCylinderPairCommonResult, error) {
	var zero FiniteCylinderPairCommonResult
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	vec := func(v geom.Vec3) [3]*big.Rat { return [3]*big.Rat{q(v.X), q(v.Y), q(v.Z)} }
	dot := func(v, w [3]*big.Rat) *big.Rat {
		s := new(big.Rat)
		for i := range v {
			s.Add(s, new(big.Rat).Mul(v[i], w[i]))
		}
		return s
	}
	cross := func(v, w [3]*big.Rat) (r [3]*big.Rat) {
		for i := range v {
			j, k := (i+1)%3, (i+2)%3
			r[i] = new(big.Rat).Sub(new(big.Rat).Mul(v[j], w[k]), new(big.Rat).Mul(v[k], w[j]))
		}
		return r
	}
	unit := func(v [3]*big.Rat) ([3]*big.Rat, bool) {
		n2 := dot(v, v)
		if n2.Sign() <= 0 {
			return v, false
		}
		n := new(big.Rat).SetFrac(new(big.Int).Sqrt(n2.Num()), new(big.Int).Sqrt(n2.Denom()))
		if new(big.Rat).Mul(n, n).Cmp(n2) != 0 {
			return v, false
		}
		for i := range v {
			v[i] = new(big.Rat).Quo(v[i], n)
		}
		return v, true
	}
	toFloat := func(v [3]*big.Rat) (geom.Vec3, error) {
		var out geom.Vec3
		for i, p := range [3]*float64{&out.X, &out.Y, &out.Z} {
			x, e := geom.RepresentedExactScalar(v[i])
			if e != nil {
				return geom.Vec3{}, e
			}
			*p = x
		}
		return out, nil
	}
	z := vec(a.Axis)
	n2 := dot(z, z)
	norm := new(big.Rat).SetFrac(new(big.Int).Sqrt(n2.Num()), new(big.Int).Sqrt(n2.Denom()))
	if new(big.Rat).Mul(norm, norm).Cmp(n2) != 0 {
		return zero, geom.ErrDomain
	}
	var basis [3][3]*big.Rat
	found := false
	for _, hint := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
		x, ok := unit(cross(vec(hint), z))
		if !ok {
			continue
		}
		for i := range x {
			x[i].Mul(x[i], norm)
		}
		y := cross(z, x)
		for i := range y {
			y[i].Quo(y[i], norm)
		}
		if _, e := toFloat(x); e != nil {
			continue
		}
		if _, e := toFloat(y); e != nil {
			continue
		}
		basis = [3][3]*big.Rat{x, y, z}
		found = true
		break
	}
	if !found {
		return zero, geom.ErrDomain
	}
	project := func(v, w [3]*big.Rat) *big.Rat { return new(big.Rat).Quo(dot(v, w), n2) }
	origin := vec(a.Origin)
	delta := vec(b.Origin)
	for i := range delta {
		delta[i].Sub(delta[i], origin[i])
	}
	x, e := geom.RepresentedExactScalar(project(delta, basis[0]))
	if e != nil {
		return zero, e
	}
	y, e := geom.RepresentedExactScalar(project(delta, basis[1]))
	if e != nil {
		return zero, e
	}
	var limits [2][2]*big.Rat
	var forward [2]bool
	for i, c := range [2]geom.RawCylinder{a, b} {
		base := new(big.Rat)
		if i == 1 {
			base = project(delta, z)
		}
		scale := project(vec(c.Axis), z)
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(scale, q(c.Min)))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(scale, q(c.Max)))
		forward[i] = scale.Sign() > 0
		if !forward[i] {
			lo, hi = hi, lo
		}
		limits[i] = [2]*big.Rat{lo, hi}
	}
	if op != Intersection && (limits[0][0].Cmp(limits[1][0]) != 0 || limits[0][1].Cmp(limits[1][1]) != 0) {
		return finiteObliqueAsymmetricTransverseSet(ctx, a, b, tolerance, op, basis, norm, x, y, limits, forward)
	}
	lo, hi := limits[0][0], limits[0][1]
	if limits[1][0].Cmp(lo) > 0 {
		lo = limits[1][0]
	}
	if limits[1][1].Cmp(hi) < 0 {
		hi = limits[1][1]
	}
	if lo.Cmp(hi) >= 0 {
		return zero, nil
	}
	at := func(cap *big.Rat) (geom.Vec3, error) {
		var p [3]*big.Rat
		for i := range p {
			p[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(z[i], cap))
		}
		return toFloat(p)
	}
	start, e := at(lo)
	if e != nil {
		return zero, e
	}
	if _, e = at(hi); e != nil {
		return zero, e
	}
	height, e := geom.RepresentedExactScalar(new(big.Rat).Sub(hi, lo))
	if e != nil {
		return zero, e
	}
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	var profiles [2]DiskProfile
	for i, c := range [2]geom.RawCylinder{a, b} {
		r, err := geom.RepresentedExactScalar(new(big.Rat).Quo(q(c.Radius), norm))
		if err != nil {
			return zero, geom.ErrDomain
		}
		profiles[i].Radius = r
	}
	profiles[1].Center = geom.Vec2{X: x, Y: y}
	localTolerance, _ := new(big.Rat).Quo(q(tolerance), norm).Float64()
	if localTolerance <= 0 || !finite(localTolerance) {
		return zero, geom.ErrUnrepresentable
	}
	var s Solid
	if op == Intersection {
		s, e = CombineParallelCylinders(f, profiles[0], profiles[1], height, op, localTolerance)
	} else {
		var accepted bool
		local := func(p DiskProfile) geom.RawCylinder {
			return geom.RawCylinder{Origin: geom.Vec3{X: p.Center.X, Y: p.Center.Y}, Axis: geom.Vec3{Z: 1}, Radius: p.Radius, Min: 0, Max: height}
		}
		s, accepted, e = finiteParallelOverlap(ctx, local(profiles[0]), local(profiles[1]), localTolerance, op)
		if e == nil && !accepted {
			e = geom.ErrDomain
		}
	}
	if e != nil {
		return zero, e
	}
	out := FiniteCylinderPairCommonResult{Construction: CylinderCommonParallelAxes}
	if op == Union {
		out.SideFaces = [2][]int{{0}, {3}}
		out.LowerCapFaces, out.UpperCapFaces = [2][]int{{1}, {4}}, [2][]int{{2}, {5}}
		if !forward[1] {
			out.LowerCapFaces[1], out.UpperCapFaces[1] = out.UpperCapFaces[1], out.LowerCapFaces[1]
		}
	}
	if op == Difference {
		out.SideFaces = [2][]int{{0}, {1}}
		out.LowerCapFaces[0], out.UpperCapFaces[0] = []int{2}, []int{3}
	}
	for id, face := range s.Faces() {
		if op != Intersection {
			break
		}
		if cylinder, ok := face.Surface.(geom.CylinderSurface); ok {
			f, r := cylinder.Parameters()
			p := f.Origin()
			for i, d := range profiles {
				if p.X == d.Center.X && p.Y == d.Center.Y && r == d.Radius {
					out.SideFaces[i] = append(out.SideFaces[i], id)
				}
			}
			continue
		}
		for i, w := range limits {
			if !face.Reversed && w[0].Cmp(lo) == 0 || face.Reversed && w[1].Cmp(hi) == 0 {
				if forward[i] != face.Reversed {
					out.LowerCapFaces[i] = append(out.LowerCapFaces[i], id)
				} else {
					out.UpperCapFaces[i] = append(out.UpperCapFaces[i], id)
				}
			}
		}
	}
	var m geom.Affine
	for j, axis := range basis {
		for i, value := range axis {
			m[i][j], _ = value.Float64() // Components were checked above or came from raw A.
		}
	}
	m[0][3], m[1][3], m[2][3] = start.X, start.Y, start.Z
	out.Solid, e = s.Transformed(m)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
