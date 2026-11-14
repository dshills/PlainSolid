package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// OffsetSideOpenDualDiskSection is an exact retained stock section for two
// circles with a common center ordinate strictly below the opening stock side.
// Owner preserves input disk order; -1 denotes an original stock edge.
// OpeningIntervals are the original disk footprints on the opening side.
type OffsetSideOpenDualDiskSection struct {
	GeneralSideOpenDualDiskSection
	OpeningIntervals [2][2]*big.Rat
}

// CertifyOffsetSideOpenDualDiskSection admits unequal radii with a common
// external center ordinate. Both side-intercept square roots and the upper
// circle crossing must be rational. The opening intervals must strictly overlap
// without containment, and the crossing must lie strictly above the opening.
// All three other stock sides strictly clear both disks. This is a sufficient
// exact subset, not a tolerance test or a claim about all side-offset placements.
// The output is a certified planar section, not a native solid or a mesh.
func CertifyOffsetSideOpenDualDiskSection(ctx context.Context, min, max geom.Vec2, discs [2]ExtrusionCircle, o trimvalidate.Options) (OffsetSideOpenDualDiskSection, error) {
	var zero OffsetSideOpenDualDiskSection
	if ctx == nil || !uvFinite(min) || !uvFinite(max) || min.X >= max.X || min.Y >= max.Y {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, disk := range discs {
		if !uvFinite(disk.Center) || !finite(disk.Radius) || disk.Radius <= 0 {
			return zero, geom.ErrInvalid
		}
		if disk.Center.Y >= min.Y || disk.Center.Y != discs[0].Center.Y {
			return zero, geom.ErrDomain
		}
	}
	if discs[0].Radius == discs[1].Radius {
		return zero, geom.ErrDomain
	}
	order := [2]int{0, 1}
	if discs[0].Center.X > discs[1].Center.X {
		order = [2]int{1, 0}
	}
	l, r := ratInset(discs[order[0]].Center.X), ratInset(discs[order[1]].Center.X)
	rl, rr := ratInset(discs[order[0]].Radius), ratInset(discs[order[1]].Radius)
	distance := new(big.Rat).Sub(r, l)
	difference := new(big.Rat).Abs(new(big.Rat).Sub(rl, rr))
	sum := new(big.Rat).Add(rl, rr)
	if distance.Cmp(difference) <= 0 || distance.Cmp(sum) >= 0 {
		return zero, geom.ErrDomain
	}
	y := ratInset(min.Y)
	centerY := ratInset(discs[0].Center.Y)
	offset := new(big.Rat).Sub(y, centerY)
	sqrt := func(v *big.Rat) (*big.Rat, error) {
		if v.Sign() <= 0 {
			return nil, geom.ErrDomain
		}
		n, d := new(big.Int).Sqrt(v.Num()), new(big.Int).Sqrt(v.Denom())
		if new(big.Int).Mul(n, n).Cmp(v.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(v.Denom()) != 0 {
			return nil, geom.ErrUnrepresentable
		}
		return new(big.Rat).SetFrac(n, d), nil
	}
	var opening [2][2]*big.Rat
	for i, disk := range discs {
		radius := ratInset(disk.Radius)
		half, e := sqrt(new(big.Rat).Sub(new(big.Rat).Mul(radius, radius), new(big.Rat).Mul(offset, offset)))
		if e != nil {
			return zero, e
		}
		opening[i] = [2]*big.Rat{new(big.Rat).Sub(ratInset(disk.Center.X), half), new(big.Rat).Add(ratInset(disk.Center.X), half)}
	}
	li, ri := opening[order[0]], opening[order[1]]
	if li[0].Cmp(ri[0]) >= 0 || ri[0].Cmp(li[1]) >= 0 || li[1].Cmp(ri[1]) >= 0 {
		return zero, geom.ErrDomain
	}
	start, end := li[0], ri[1]
	topL, topR := new(big.Rat).Add(centerY, rl), new(big.Rat).Add(centerY, rr)
	if new(big.Rat).Sub(l, rl).Cmp(ratInset(min.X)) <= 0 || new(big.Rat).Add(r, rr).Cmp(ratInset(max.X)) >= 0 || topL.Cmp(ratInset(max.Y)) >= 0 || topR.Cmp(ratInset(max.Y)) >= 0 {
		return zero, geom.ErrDomain
	}
	a := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(distance, distance), new(big.Rat).Sub(new(big.Rat).Mul(rl, rl), new(big.Rat).Mul(rr, rr))), new(big.Rat).Mul(big.NewRat(2, 1), distance))
	h2 := new(big.Rat).Sub(new(big.Rat).Mul(rl, rl), new(big.Rat).Mul(a, a))
	if h2.Sign() <= 0 {
		return zero, geom.ErrDomain
	}
	n, d := new(big.Int).Sqrt(h2.Num()), new(big.Int).Sqrt(h2.Denom())
	if new(big.Int).Mul(n, n).Cmp(h2.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(h2.Denom()) != 0 {
		return zero, geom.ErrUnrepresentable
	}
	// On the common x projection, hL²-hR²=2*distance*(joinX-x).
	// Thus the left circle is the union envelope before the join, and the
	// right circle afterwards, even when the join lies outside the centers.
	join := [2]*big.Rat{new(big.Rat).Add(l, a), new(big.Rat).Add(centerY, new(big.Rat).SetFrac(n, d))}
	if join[1].Cmp(y) <= 0 || join[0].Cmp(start) <= 0 || join[0].Cmp(end) >= 0 {
		return zero, geom.ErrDomain
	}
	points := [][2]*big.Rat{{ratInset(min.X), y}}
	owners := []int{}
	add := func(p [2]*big.Rat, owner int) { points = append(points, p); owners = append(owners, owner) }
	add([2]*big.Rat{start, y}, -1)
	if l.Cmp(join[0]) < 0 {
		add([2]*big.Rat{l, topL}, order[0])
	}
	add(join, order[0])
	if r.Cmp(join[0]) > 0 {
		add([2]*big.Rat{r, topR}, order[1])
	}
	add([2]*big.Rat{end, y}, order[1])
	add([2]*big.Rat{ratInset(max.X), y}, -1)
	add([2]*big.Rat{ratInset(max.X), ratInset(max.Y)}, -1)
	add([2]*big.Rat{ratInset(min.X), ratInset(max.Y)}, -1)
	owners = append(owners, -1)
	out := OffsetSideOpenDualDiskSection{OpeningIntervals: opening, GeneralSideOpenDualDiskSection: GeneralSideOpenDualDiskSection{Owner: owners, Join: join, XIntervals: [2][2]*big.Rat{{start, join[0]}, {join[0], end}}}}
	var loop trimvalidate.Loop
	for i, p := range points {
		z := points[(i+1)%len(points)]
		var q ExactQuadraticUV
		var e error
		if owners[i] < 0 {
			q, e = quadraticLine(p, z)
		} else {
			disk := discs[owners[i]]
			q, e = shortRationalCircleArc(p, z, [2]*big.Rat{ratInset(disk.Center.X), centerY}, ratInset(disk.Radius))
		}
		if e != nil {
			return zero, e
		}
		segment, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
		if e != nil {
			return zero, e
		}
		loop = append(loop, segment)
		out.Outer = append(out.Outer, q)
	}
	proof, e := trimvalidate.ValidateLoops(ctx, loop, nil, o)
	if e != nil {
		return zero, e
	}
	if proof.Status != trimvalidate.Certified {
		return zero, geom.ErrConvergence
	}
	return out, nil
}
