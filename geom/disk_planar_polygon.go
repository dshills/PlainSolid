package geom

import (
	"context"
	"math/big"
)

type DiskPolygonRelation uint8

const (
	DiskPolygonStrictlyCovered DiskPolygonRelation = iota
	DiskPolygonCoveredTouching
	DiskPolygonOutside
	DiskPolygonOutsideTouching
	DiskPolygonOverlap
)

type DiskPolygonOptions struct{ MaxEdges, MaxOperations, MaxCoefficientBits int }

// DiskPolygonComparison concerns a closed filled outer polygon minus open hole
// interiors. BoundaryCenter means the center lies on a region boundary. The
// minimum squared distance is to all original 3D boundary segments.
type DiskPolygonComparison struct {
	Complete                       bool
	Relation                       DiskPolygonRelation
	CenterInside, BoundaryCenter   bool
	MinimumBoundaryDistanceSquared *big.Rat
	ExactOperations                int
}
type diskPolygonVertex struct {
	world [3]*big.Rat
	xy    [2]*big.Rat
}

// ClassifyDiskPlanarPolygon validates a simple planar polygon and optional
// strictly interior, mutually disjoint, nonnested simple holes. Loops omit the
// repeated closing vertex; winding direction is arbitrary. All vertices must
// lie exactly in the disk's raw plane. Noncoplanarity returns ErrDomain rather
// than a spatial disjointness claim. Limits include quadratic simplicity checks.
func ClassifyDiskPlanarPolygon(ctx context.Context, disk PlanarCircularProfile, outer []Vec3, holes [][]Vec3, o DiskPolygonOptions) (out DiskPolygonComparison, err error) {
	if ctx == nil || !disk.valid() {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEdges < 3 || o.MaxEdges > 4096 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = budget.operations }()
	vector := func(p Vec3) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(p.Z)}
	}
	center, normal := vector(disk.center), vector(disk.normal)
	for _, v := range [][3]*big.Rat{center, normal} {
		for _, q := range v {
			if e := budget.check(q); e != nil {
				return out, e
			}
		}
	}
	drop := 0
	for normal[drop].Sign() == 0 {
		drop++
	}
	axes := [2]int{(drop + 1) % 3, (drop + 2) % 3}
	query := [2]*big.Rat{center[axes[0]], center[axes[1]]}
	input := append([][]Vec3{outer}, holes...)
	count := 0
	loops := make([][]diskPolygonVertex, len(input))
	for l, loop := range input {
		if len(loop) < 3 || len(loop) > o.MaxEdges-count {
			return out, ErrInvalid
		}
		count += len(loop)
		for _, p := range loop {
			if !p.IsFinite() {
				return out, ErrInvalid
			}
			v := diskPolygonVertex{world: vector(p)}
			dot := new(big.Rat)
			for i, q := range v.world {
				if e := budget.check(q); e != nil {
					return out, e
				}
				dot.Add(dot, new(big.Rat).Mul(normal[i], new(big.Rat).Sub(q, center[i])))
			}
			if e := budget.check(dot); e != nil {
				return out, e
			}
			if dot.Sign() != 0 {
				return out, ErrDomain
			}
			v.xy = [2]*big.Rat{v.world[axes[0]], v.world[axes[1]]}
			loops[l] = append(loops[l], v)
		}
	}
	orient := func(a, b, c [2]*big.Rat) (int, error) {
		q := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
		if e := budget.check(q); e != nil {
			return 0, e
		}
		return q.Sign(), nil
	}
	on := func(a, b, c [2]*big.Rat) bool {
		for i := 0; i < 2; i++ {
			lo, hi := a[i], b[i]
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if c[i].Cmp(lo) < 0 || c[i].Cmp(hi) > 0 {
				return false
			}
		}
		return true
	}
	intersect := func(a, b, c, d [2]*big.Rat) (bool, error) {
		abC, e := orient(a, b, c)
		if e != nil {
			return false, e
		}
		abD, e := orient(a, b, d)
		if e != nil {
			return false, e
		}
		cdA, e := orient(c, d, a)
		if e != nil {
			return false, e
		}
		cdB, e := orient(c, d, b)
		if e != nil {
			return false, e
		}
		return abC*abD < 0 && cdA*cdB < 0 || (abC == 0 && on(a, b, c)) || (abD == 0 && on(a, b, d)) || (cdA == 0 && on(c, d, a)) || (cdB == 0 && on(c, d, b)), nil
	}
	locate := func(loop []diskPolygonVertex, p [2]*big.Rat) (inside, boundary bool, err error) {
		winding := 0
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			sign, e := orient(a.xy, b.xy, p)
			if e != nil {
				return false, false, e
			}
			if sign == 0 && on(a.xy, b.xy, p) {
				return false, true, nil
			}
			if a.xy[1].Cmp(p[1]) <= 0 && b.xy[1].Cmp(p[1]) > 0 && sign > 0 {
				winding++
			}
			if a.xy[1].Cmp(p[1]) > 0 && b.xy[1].Cmp(p[1]) <= 0 && sign < 0 {
				winding--
			}
		}
		return winding != 0, false, nil
	}
	for l, loop := range loops {
		area := new(big.Rat)
		for i, a := range loop {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			b := loop[(i+1)%len(loop)]
			if a.xy[0].Cmp(b.xy[0]) == 0 && a.xy[1].Cmp(b.xy[1]) == 0 {
				return out, ErrInvalid
			}
			area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a.xy[0], b.xy[1]), new(big.Rat).Mul(a.xy[1], b.xy[0])))
			previous := loop[(i+len(loop)-1)%len(loop)]
			sign, e := orient(previous.xy, a.xy, b.xy)
			if e != nil {
				return out, e
			}
			if sign == 0 && (on(previous.xy, a.xy, b.xy) || on(a.xy, b.xy, previous.xy)) {
				return out, ErrInvalid
			}
			for m := l; m < len(loops); m++ {
				for j, c := range loops[m] {
					if m == l && (j <= i || j == (i+1)%len(loop) || (i == 0 && j == len(loop)-1)) {
						continue
					}
					d := loops[m][(j+1)%len(loops[m])]
					hit, e := intersect(a.xy, b.xy, c.xy, d.xy)
					if e != nil {
						return out, e
					}
					if hit {
						return out, ErrInvalid
					}
				}
			}
		}
		if e := budget.check(area); e != nil {
			return out, e
		}
		if area.Sign() == 0 {
			return out, ErrDegenerate
		}
	}
	for i := 1; i < len(loops); i++ {
		inside, boundary, e := locate(loops[0], loops[i][0].xy)
		if e != nil {
			return out, e
		}
		if !inside || boundary {
			return out, ErrInvalid
		}
		for j := 1; j < len(loops); j++ {
			if i == j {
				continue
			}
			inside, boundary, e := locate(loops[j], loops[i][0].xy)
			if e != nil {
				return out, e
			}
			if inside || boundary {
				return out, ErrInvalid
			}
		}
	}
	inside, boundary, e := locate(loops[0], query)
	if e != nil {
		return out, e
	}
	for i := 1; i < len(loops); i++ {
		hole, onHole, e := locate(loops[i], query)
		if e != nil {
			return out, e
		}
		if hole {
			inside = false
		}
		boundary = boundary || onHole
	}
	out.CenterInside = inside && !boundary
	out.BoundaryCenter = boundary
	for _, loop := range loops {
		for i, a := range loop {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			b := loop[(i+1)%len(loop)]
			var delta, from [3]*big.Rat
			den, num := new(big.Rat), new(big.Rat)
			for k := 0; k < 3; k++ {
				delta[k] = new(big.Rat).Sub(b.world[k], a.world[k])
				from[k] = new(big.Rat).Sub(center[k], a.world[k])
				den.Add(den, new(big.Rat).Mul(delta[k], delta[k]))
				num.Add(num, new(big.Rat).Mul(from[k], delta[k]))
			}
			parameter := new(big.Rat).Quo(num, den)
			if parameter.Sign() < 0 {
				parameter.SetInt64(0)
			} else if parameter.Cmp(big.NewRat(1, 1)) > 0 {
				parameter.SetInt64(1)
			}
			distance := new(big.Rat)
			for k := 0; k < 3; k++ {
				d := new(big.Rat).Sub(from[k], new(big.Rat).Mul(parameter, delta[k]))
				distance.Add(distance, new(big.Rat).Mul(d, d))
			}
			if e := budget.check(distance); e != nil {
				return out, e
			}
			if out.MinimumBoundaryDistanceSquared == nil || distance.Cmp(out.MinimumBoundaryDistanceSquared) < 0 {
				out.MinimumBoundaryDistanceSquared = distance
			}
		}
	}
	radius := new(big.Rat).SetFloat64(disk.radius)
	radius.Mul(radius, radius)
	if e := budget.check(radius); e != nil {
		return out, e
	}
	cmp := out.MinimumBoundaryDistanceSquared.Cmp(radius)
	if boundary || cmp < 0 {
		out.Relation = DiskPolygonOverlap
	} else if out.CenterInside {
		out.Relation = DiskPolygonStrictlyCovered
		if cmp == 0 {
			out.Relation = DiskPolygonCoveredTouching
		}
	} else {
		out.Relation = DiskPolygonOutside
		if cmp == 0 {
			out.Relation = DiskPolygonOutsideTouching
		}
	}
	out.Complete = true
	return out, nil
}
