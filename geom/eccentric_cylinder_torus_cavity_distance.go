package geom

import (
	"context"
	"math/big"
)

// EccentricCylinderTorusCavityDistance adds a complete exact owner mask only when all
// possible support minima collapse to the same exact rational distance.
// Zero ExactOwners means unresolved ties, not absence of a nearest point.
type EccentricCylinderTorusCavityDistance struct {
	TorusCylinderDistance
	ExactOwners  BoreBoundaryMask
	ChartOrigins [4][2]*big.Rat
}

// BoundBoundaryDistance uses two independent radial charts and covers: the whole inward torus,
// finite outward wall and both cap disks. Witnesses are attained support points
// (or azimuth circles), not necessarily exact minimizers. PossibleOwners is
// conservative; only strict separation establishes UniqueOwner. Arithmetic,
// cells and precision use TorusCylinderDistanceOptions with atomic failure.
func (c EccentricCylinderTorusCavityMaterial) BoundBoundaryDistance(ctx context.Context, p [3]*big.Rat, o TorusCylinderDistanceOptions) (out EccentricCylinderTorusCavityDistance, err error) {
	s := c.source
	defer func() {
		if err != nil {
			out = EccentricCylinderTorusCavityDistance{}
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
	for i := range out.ChartOrigins {
		for j := 0; j < 2; j++ {
			out.ChartOrigins[i][j] = new(big.Rat)
		}
	}
	for j := 0; j < 2; j++ {
		a.check(c.offset[j])
		out.ChartOrigins[0][j] = new(big.Rat).Set(c.offset[j])
	}
	tx, ty := a.sub(p[0], c.offset[0]), a.sub(p[1], c.offset[1])
	w.rho2 = a.add(a.mul(tx, tx), a.mul(ty, ty))
	w.whole = w.rho2.Sign() == 0
	w.rho, e = w.sqrt(w.rho2)
	if e != nil {
		return out, e
	}
	r2 := a.mul(s.r, s.r)
	c2 := a.mul(s.c, s.c)
	left, right := a.sub(s.R, s.r), a.add(s.R, s.r)
	// The full meridian circle has exact distance |sqrt((rho-R)^2+z^2)-r|.
	// Its nonnegative radial range R-r ensures the aligned azimuth foot exists.
	dr := w.square([2]*big.Rat{a.sub(w.rho[0], s.R), a.sub(w.rho[1], s.R)})
	dlo, e := w.sqrt(a.add(dr[0], a.mul(w.z, w.z)))
	if e != nil {
		return out, e
	}
	dhi, e := w.sqrt(a.add(dr[1], a.mul(w.z, w.z)))
	if e != nil {
		return out, e
	}
	analyticLower := w.square([2]*big.Rat{a.sub(dlo[0], s.r), a.sub(dhi[1], s.r)})[0]
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
		if analyticLower.Cmp(lower) > 0 {
			lower = analyticLower
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
	// The cylinder is NOT translated: restore its independent radial chart.
	w.rho2 = a.add(a.mul(p[0], p[0]), a.mul(p[1], p[1]))
	w.whole = w.rho2.Sign() == 0
	w.rho, e = w.sqrt(w.rho2)
	if e != nil {
		return out, e
	}
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
	for _, z := range []*big.Rat{s.lower, s.upper} {
		if e := addWall(z, new(big.Rat), 0); e != nil {
			return out, e
		}
	}
	inside := w.z.Cmp(s.lower) >= 0 && w.z.Cmp(s.upper) <= 0
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
	if out.Lower.Cmp(out.Upper) == 0 {
		complete := true
		var mask BoreBoundaryMask
		for _, v := range out.Supports {
			if v.Lower.Cmp(out.Upper) <= 0 {
				if v.Lower.Cmp(v.Upper) != 0 || v.Upper.Cmp(out.Upper) != 0 {
					complete = false
					break
				}
				mask |= v.Owner
			}
		}
		if complete {
			out.ExactOwners = mask
		}
	}
	out.ExactOperations = b.operations
	return out, nil
}
