package geom

import (
	"context"
	"math/big"
)

// TorusCylinderDistanceOptions bounds all retained-support distance work.
// SquaredTolerance is positive. MaxCells defaults4096 (maximum65536), and
// PrecisionBits defaults64 (16..256). Failure returns no partial proof.
type TorusCylinderDistanceOptions struct {
	SquaredTolerance        *big.Rat
	MaxCells, PrecisionBits int
	Arithmetic              SurfacePatchOptions
}

// TorusCylinderDistanceWitness is an attained original-support point or circle:
// rho=sqrt(RadialSquared), z=HeightBase+HeightSign*sqrt(HeightRadicand).
// Off-axis its azimuth equals the original query's; on-axis WholeCircle denotes
// every azimuth (at rho=0 this collapses to one point). It is an upper-bound
// witness, NOT necessarily a stationary or exact nearest point.
type TorusCylinderDistanceWitness struct {
	RadialSquared, HeightBase, HeightRadicand *big.Rat
	HeightSign                                int
	WholeCircle                               bool
}
type TorusCylinderSupportDistance struct {
	Owner        BoreBoundaryMask
	Lower, Upper *big.Rat
	Witness      TorusCylinderDistanceWitness
}

// TorusCylinderDistance encloses the complete boundary minimum. Supports are
// original torus/wall/lowercap/uppercap, with zero Owner for absent supports.
// PossibleOwners is conservative; UniqueOwner is set only by strict interval
// separation from EVERY other exposed support. No equality/tie is inferred.
type TorusCylinderDistance struct {
	Lower, Upper                *big.Rat
	Supports                    [4]TorusCylinderSupportDistance
	PossibleOwners, UniqueOwner BoreBoundaryMask
	Cells, ExactOperations      int
}

type torusDistanceWork struct {
	a       sphereOverlapArithmetic
	bits    int
	rho2, z *big.Rat
	rho     [2]*big.Rat
	whole   bool
}

func (w *torusDistanceWork) sqrt(q *big.Rat) ([2]*big.Rat, error) {
	if w.a.err != nil {
		return [2]*big.Rat{}, w.a.err
	}
	l, h, e := rationalSqrtBounds(q, w.bits, w.a.b)
	return [2]*big.Rat{l, h}, e
}
func (w *torusDistanceWork) square(v [2]*big.Rat) [2]*big.Rat {
	a := &w.a
	l, h := a.mul(v[0], v[0]), a.mul(v[1], v[1])
	if l.Cmp(h) > 0 {
		l, h = h, l
	}
	if v[0].Sign() <= 0 && v[1].Sign() >= 0 {
		l = new(big.Rat)
	}
	return [2]*big.Rat{l, h}
}
func (w *torusDistanceWork) witness(rho2, base, rad *big.Rat, sign int) (TorusCylinderDistanceWitness, [2]*big.Rat, error) {
	a := &w.a
	rho, e := w.sqrt(rho2)
	if e != nil {
		return TorusCylinderDistanceWitness{}, [2]*big.Rat{}, e
	}
	z := [2]*big.Rat{new(big.Rat).Set(base), new(big.Rat).Set(base)}
	if sign != 0 {
		h, e := w.sqrt(rad)
		if e != nil {
			return TorusCylinderDistanceWitness{}, [2]*big.Rat{}, e
		}
		if sign < 0 {
			z = [2]*big.Rat{a.sub(base, h[1]), a.sub(base, h[0])}
		} else {
			z = [2]*big.Rat{a.add(base, h[0]), a.add(base, h[1])}
		}
	}
	dr := w.square([2]*big.Rat{a.sub(w.rho[0], rho[1]), a.sub(w.rho[1], rho[0])})
	if rho2.Cmp(w.rho2) == 0 {
		dr = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	}
	dz := w.square([2]*big.Rat{a.sub(w.z, z[1]), a.sub(w.z, z[0])})
	bounds := [2]*big.Rat{a.add(dr[0], dz[0]), a.add(dr[1], dz[1])}
	v := TorusCylinderDistanceWitness{new(big.Rat).Set(rho2), new(big.Rat).Set(base), new(big.Rat).Set(rad), sign, w.whole}
	return v, bounds, a.err
}

// BoundBoundaryDistance reduces the full 3D boundary to the retained meridian:
// azimuth alignment minimizes distance for rho>=0. Both torus height branches
// are covered by rational radial cells with exact sqrt envelopes. Box distance
// bounds every point below, while actual analytic samples bound above. Walls
// and disks use exact meridian clamping. This covers all retained original
// supports, including hidden wall gaps in Union and circle families on axis.
func (s TorusCylinderMaterial) BoundBoundaryDistance(ctx context.Context, p [3]*big.Rat, o TorusCylinderDistanceOptions) (out TorusCylinderDistance, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderDistance{}
		}
	}()
	if ctx == nil || s.R == nil || o.SquaredTolerance == nil || o.SquaredTolerance.Sign() <= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, x := range p {
		if x == nil {
			return out, ErrInvalid
		}
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 || o.PrecisionBits < 16 || o.PrecisionBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	w := torusDistanceWork{a: sphereOverlapArithmetic{ctx: ctx, b: b}, bits: o.PrecisionBits, z: p[2]}
	a := &w.a
	for _, q := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper, p[0], p[1], p[2], o.SquaredTolerance} {
		a.check(q)
	}
	w.rho2 = a.add(a.mul(p[0], p[0]), a.mul(p[1], p[1]))
	w.whole = w.rho2.Sign() == 0
	w.rho, e = w.sqrt(w.rho2)
	if e != nil {
		return out, e
	}
	r2 := a.mul(s.r, s.r)
	c2 := a.mul(s.c, s.c)
	delta := a.sub(s.c, s.R)
	h2 := a.sub(r2, a.mul(delta, delta))
	// Whole retained torus strip, with both signs of the meridian height.
	left, right := a.sub(s.R, s.r), s.c
	if s.op != TorusCylinderCommon {
		left, right = s.c, a.add(s.R, s.r)
	}
	type cell struct{ l, h, lower *big.Rat }
	best := TorusCylinderSupportDistance{Owner: TorusCylinderTorus}
	makeCell := func(l, h *big.Rat) (cell, error) {
		ql := a.sub(r2, a.mul(a.sub(l, s.R), a.sub(l, s.R)))
		qh := a.sub(r2, a.mul(a.sub(h, s.R), a.sub(h, s.R)))
		low, high := ql, qh
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		if l.Cmp(s.R) <= 0 && h.Cmp(s.R) >= 0 {
			high = r2
		}
		zl, e := w.sqrt(low)
		if e != nil {
			return cell{}, e
		}
		zh, e := w.sqrt(high)
		if e != nil {
			return cell{}, e
		}
		radial := w.square([2]*big.Rat{a.sub(w.rho[0], h), a.sub(w.rho[1], l)})[0]
		var lower *big.Rat
		for _, sign := range []int{-1, 1} {
			zlo, zhi := zl[0], zh[1]
			if sign < 0 {
				zlo, zhi = a.sub(new(big.Rat), zhi), a.sub(new(big.Rat), zlo)
			}
			dz := w.square([2]*big.Rat{a.sub(w.z, zhi), a.sub(w.z, zlo)})[0]
			v := a.add(radial, dz)
			if lower == nil || v.Cmp(lower) < 0 {
				lower = v
			}
			mid := a.div(a.add(l, h), big.NewRat(2, 1))
			for _, rho := range []*big.Rat{l, mid, h} {
				rad := a.sub(r2, a.mul(a.sub(rho, s.R), a.sub(rho, s.R)))
				witness, bounds, e := w.witness(a.mul(rho, rho), new(big.Rat), rad, sign)
				if e != nil {
					return cell{}, e
				}
				if best.Upper == nil || bounds[1].Cmp(best.Upper) < 0 {
					best.Upper = bounds[1]
					best.Witness = witness
				}
			}
		}
		return cell{new(big.Rat).Set(l), new(big.Rat).Set(h), lower}, a.err
	}
	first, e := makeCell(left, right)
	if e != nil {
		return out, e
	}
	cells := []cell{first}
	for {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		at := 0
		for i := range cells {
			a.check(cells[i].lower)
			if cells[i].lower.Cmp(cells[at].lower) < 0 {
				at = i
			}
		}
		best.Lower = cells[at].lower
		if a.sub(best.Upper, best.Lower).Cmp(o.SquaredTolerance) <= 0 {
			break
		}
		if a.err != nil {
			return out, a.err
		}
		if len(cells) >= o.MaxCells {
			return out, ErrConvergence
		}
		c := cells[at]
		mid := a.div(a.add(c.l, c.h), big.NewRat(2, 1))
		l, e := makeCell(c.l, mid)
		if e != nil {
			return out, e
		}
		h, e := makeCell(mid, c.h)
		if e != nil {
			return out, e
		}
		cells[at] = l
		cells = append(cells, h)
	}
	out.Supports[0] = best
	out.Cells = len(cells)
	// Exact wall projection: include segment endpoints and query z only when
	// its exact sign and square prove membership in an exposed wall segment.
	wall := TorusCylinderSupportDistance{Owner: TorusCylinderWall}
	addWall := func(base, rad *big.Rat, sign int) error {
		v, d, e := w.witness(c2, base, rad, sign)
		if e != nil {
			return e
		}
		if wall.Lower == nil || d[0].Cmp(wall.Lower) < 0 {
			wall.Lower = d[0]
		}
		if wall.Upper == nil || d[1].Cmp(wall.Upper) < 0 {
			wall.Upper = d[1]
			wall.Witness = v
		}
		return nil
	}
	for _, sign := range []int{-1, 1} {
		if e := addWall(new(big.Rat), h2, sign); e != nil {
			return out, e
		}
	}
	z2 := a.mul(w.z, w.z)
	inside := z2.Cmp(h2) <= 0
	if s.op == TorusCylinderUnion {
		inside = !inside && w.z.Cmp(s.lower) >= 0 && w.z.Cmp(s.upper) <= 0
		for _, z := range []*big.Rat{s.lower, s.upper} {
			if e := addWall(z, new(big.Rat), 0); e != nil {
				return out, e
			}
		}
	}
	if inside {
		if e := addWall(w.z, new(big.Rat), 0); e != nil {
			return out, e
		}
	}
	out.Supports[1] = wall
	if s.op == TorusCylinderUnion {
		rho2 := w.rho2
		if rho2.Cmp(c2) > 0 {
			rho2 = c2
		}
		for i, z := range []*big.Rat{s.lower, s.upper} {
			v, d, e := w.witness(rho2, z, new(big.Rat), 0)
			if e != nil {
				return out, e
			}
			out.Supports[i+2] = TorusCylinderSupportDistance{BoreBoundaryMask(4 << uint(i)), d[0], d[1], v}
		}
	}
	for _, v := range out.Supports {
		if v.Owner == 0 {
			continue
		}
		if a.sub(v.Upper, v.Lower).Cmp(o.SquaredTolerance) > 0 {
			return out, ErrConvergence
		}
		if out.Lower == nil || v.Lower.Cmp(out.Lower) < 0 {
			out.Lower = new(big.Rat).Set(v.Lower)
		}
		if out.Upper == nil || v.Upper.Cmp(out.Upper) < 0 {
			out.Upper = new(big.Rat).Set(v.Upper)
		}
	}
	for _, v := range out.Supports {
		if v.Owner != 0 && v.Lower.Cmp(out.Upper) <= 0 {
			out.PossibleOwners |= v.Owner
		}
	}
	if out.PossibleOwners != 0 && (out.PossibleOwners&(out.PossibleOwners-1)) == 0 {
		out.UniqueOwner = out.PossibleOwners
	}
	if a.err != nil {
		return out, a.err
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ExactOperations = b.operations
	return out, nil
}
