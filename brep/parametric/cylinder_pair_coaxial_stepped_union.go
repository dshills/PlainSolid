package parametric

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// finiteCoaxialSteppedCylinderUnion takes the outer disk on each exact axial
// slab. The revolved meridian retains the original cylindrical walls and any
// exposed part of an original cap at an axial step.
func finiteCoaxialSteppedCylinderUnion(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	axis, sa := coordinateCylinderAxis(a.Axis)
	_, sb := coordinateCylinderAxis(b.Axis)
	if sa == 0 || sb == 0 || a.Radius == b.Radius {
		return zero, false, nil
	}
	aPos, bPos := [3]float64{a.Origin.X, a.Origin.Y, a.Origin.Z}, [3]float64{b.Origin.X, b.Origin.Y, b.Origin.Z}
	for i, d := range [3]float64{axis.X, axis.Y, axis.Z} {
		if d == 0 && aPos[i] != bPos[i] {
			return zero, false, nil
		}
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	operands := [2]geom.RawCylinder{a, b}
	var spans [2][2]*big.Rat
	var increasing [2]bool
	ends := make([]*big.Rat, 0, 4)
	for i, c := range operands {
		x, speed := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		for j, t := range [2]float64{c.Min, c.Max} {
			spans[i][j] = new(big.Rat).Add(x, new(big.Rat).Mul(speed, q(t)))
		}
		increasing[i] = speed.Sign() > 0
		if !increasing[i] {
			spans[i][0], spans[i][1] = spans[i][1], spans[i][0]
		}
		ends = append(ends, spans[i][:]...)
	}
	if spans[0][1].Cmp(spans[1][0]) < 0 || spans[1][1].Cmp(spans[0][0]) < 0 {
		return zero, false, nil
	}
	sort.Slice(ends, func(i, j int) bool { return ends[i].Cmp(ends[j]) < 0 })
	start := a.Origin
	for i, dir := range [3]float64{axis.X, axis.Y, axis.Z} {
		if dir == 0 {
			continue
		}
		p, e := geom.RepresentedExactScalar(new(big.Rat).Mul(ends[0], q(dir)))
		if e != nil {
			return zero, true, e
		}
		*[3]*float64{&start.X, &start.Y, &start.Z}[i] = p
	}
	frame, err := geom.FrameFromNormal(start, axis)
	if err != nil {
		return zero, true, err
	}
	type slab struct {
		lo, hi *big.Rat
		owner  int
	}
	var slabs []slab
	common := new(big.Rat)
	for i, lo := range ends[:len(ends)-1] {
		hi := ends[i+1]
		if lo.Cmp(hi) == 0 {
			continue
		}
		owner, count := -1, 0
		for j, span := range spans {
			if lo.Cmp(span[0]) >= 0 && hi.Cmp(span[1]) <= 0 {
				count++
				if owner < 0 || operands[j].Radius > operands[owner].Radius {
					owner = j
				}
			}
		}
		if owner < 0 {
			return zero, true, geom.ErrDomain
		}
		if count == 2 {
			common.Add(common, new(big.Rat).Sub(hi, lo))
		}
		if n := len(slabs); n != 0 && slabs[n-1].owner == owner {
			slabs[n-1].hi = hi
		} else {
			slabs = append(slabs, slab{lo, hi, owner})
		}
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if q(min(a.Radius, b.Radius)).Cmp(pad) <= 0 || new(big.Rat).Sub(q(max(a.Radius, b.Radius)), q(min(a.Radius, b.Radius))).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	local := func(x *big.Rat) (float64, error) {
		if _, err := geom.RepresentedExactScalar(x); err != nil {
			return 0, err
		}
		return geom.RepresentedExactScalar(new(big.Rat).Sub(x, ends[0]))
	}
	var out FiniteCylinderPairUnionResult
	profile := []geom.Vec2{{}}
	cap := func(owner, modelEnd int) {
		groups := &out.LowerCapFaces
		if (modelEnd == 1) == increasing[owner] {
			groups = &out.UpperCapFaces
		}
		groups[owner] = append(groups[owner], len(profile)-1)
	}
	for i, s := range slabs {
		if new(big.Rat).Sub(s.hi, s.lo).Cmp(pad) <= 0 {
			return zero, true, geom.ErrDomain
		}
		lo, err := local(s.lo)
		if err != nil {
			return zero, true, err
		}
		hi, err := local(s.hi)
		if err != nil {
			return zero, true, err
		}
		r := operands[s.owner].Radius
		if i == 0 {
			cap(s.owner, 0)
		} else if prev := slabs[i-1].owner; operands[prev].Radius > r {
			cap(prev, 1)
		} else {
			cap(s.owner, 0)
		}
		profile = append(profile, geom.Vec2{X: r, Y: lo})
		out.WallFaces[s.owner] = append(out.WallFaces[s.owner], len(profile)-1)
		profile = append(profile, geom.Vec2{X: r, Y: hi})
		if i == len(slabs)-1 {
			cap(s.owner, 1)
			profile = append(profile, geom.Vec2{Y: hi})
		}
	}
	out.Solid, err = Revolve(frame, profile)
	if err != nil {
		return zero, true, err
	}
	if err = ctx.Err(); err != nil {
		return zero, true, err
	}
	if common.Sign() == 0 {
		out.CommonVolumeSquared = common
	} else {
		r := q(min(a.Radius, b.Radius))
		out.CommonVolumePiCoefficient = common.Mul(common, new(big.Rat).Mul(r, r))
	}
	return out, true, nil
}
