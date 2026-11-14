package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// UnequalCenterSideOpenDualDiskSection retains exact proposed upper and
// excluded lower circle intersections. OpeningIntervals use original owners.
// The caller owns all returned rational values.
type UnequalCenterSideOpenDualDiskSection struct {
	GeneralSideOpenDualDiskSection
	OpeningIntervals [2][2]*big.Rat
	LowerJoin        [2]*big.Rat
}

// CertifyUnequalCenterSideOpenDualDiskSection verifies a supplied rational join
// for unequal-radius disks with distinct center ordinates strictly below the
// opening side. Both rational side footprints overlap without containment;
// exactly one circle intersection is above the side. Complete disks strictly
// clear other stock sides. A supplied join is evidence to check, never a hint
// to snap. This API certifies planar geometry only, not a solid.
func CertifyUnequalCenterSideOpenDualDiskSection(ctx context.Context, min, max geom.Vec2, discs [2]ExtrusionCircle, proposedJoin [2]*big.Rat, o trimvalidate.Options) (UnequalCenterSideOpenDualDiskSection, error) {
	var zero UnequalCenterSideOpenDualDiskSection
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
		if disk.Center.Y >= min.Y {
			return zero, geom.ErrDomain
		}
	}
	if discs[0].Radius == discs[1].Radius || discs[0].Center.Y == discs[1].Center.Y {
		return zero, geom.ErrDomain
	}
	maxBits := o.MaxBits
	if maxBits == 0 {
		maxBits = 4096
	}
	if maxBits < 1 || maxBits > 1048576 {
		return zero, geom.ErrInvalid
	}
	var join [2]*big.Rat
	for i, q := range proposedJoin {
		if q == nil {
			return zero, geom.ErrInvalid
		}
		if q.Num().BitLen() > maxBits || q.Denom().BitLen() > maxBits {
			return zero, geom.ErrConvergence
		}
		join[i] = new(big.Rat).Set(q)
	}
	order := [2]int{0, 1}
	if discs[0].Center.X > discs[1].Center.X {
		order = [2]int{1, 0}
	}
	l, r := ratInset(discs[order[0]].Center.X), ratInset(discs[order[1]].Center.X)
	rl, rr := ratInset(discs[order[0]].Radius), ratInset(discs[order[1]].Radius)
	y := ratInset(min.Y)
	for _, disk := range discs {
		dx := new(big.Rat).Sub(join[0], ratInset(disk.Center.X))
		dy := new(big.Rat).Sub(join[1], ratInset(disk.Center.Y))
		radius := ratInset(disk.Radius)
		if new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)).Cmp(new(big.Rat).Mul(radius, radius)) != 0 {
			return zero, geom.ErrDomain
		}
		if new(big.Rat).Sub(ratInset(disk.Center.X), radius).Cmp(ratInset(min.X)) <= 0 || new(big.Rat).Add(ratInset(disk.Center.X), radius).Cmp(ratInset(max.X)) >= 0 || new(big.Rat).Add(ratInset(disk.Center.Y), radius).Cmp(ratInset(max.Y)) >= 0 {
			return zero, geom.ErrDomain
		}
	}
	// Reflect the proposed crossing across the exact line of centers. The
	// radical-axis midpoint lies on that line; reflection gives the other root.
	delta := [2]*big.Rat{new(big.Rat).Sub(ratInset(discs[1].Center.X), ratInset(discs[0].Center.X)), new(big.Rat).Sub(ratInset(discs[1].Center.Y), ratInset(discs[0].Center.Y))}
	norm := new(big.Rat).Add(new(big.Rat).Mul(delta[0], delta[0]), new(big.Rat).Mul(delta[1], delta[1]))
	if norm.Sign() == 0 {
		return zero, geom.ErrDomain
	}
	r0, r1 := ratInset(discs[0].Radius), ratInset(discs[1].Radius)
	factor := new(big.Rat).Quo(new(big.Rat).Add(norm, new(big.Rat).Sub(new(big.Rat).Mul(r0, r0), new(big.Rat).Mul(r1, r1))), new(big.Rat).Mul(big.NewRat(2, 1), norm))
	var lower [2]*big.Rat
	for i, c := range []float64{discs[0].Center.X, discs[0].Center.Y} {
		mid := new(big.Rat).Add(ratInset(c), new(big.Rat).Mul(factor, delta[i]))
		lower[i] = new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), mid), join[i])
	}
	if join[1].Cmp(y) <= 0 || lower[1].Cmp(y) >= 0 {
		return zero, geom.ErrDomain
	}
	// Distinct intersections imply strict transversality of the two circles.
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
		offset := new(big.Rat).Sub(y, ratInset(disk.Center.Y))
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
	topL, topR := new(big.Rat).Add(ratInset(discs[order[0]].Center.Y), rl), new(big.Rat).Add(ratInset(discs[order[1]].Center.Y), rr)
	if join[0].Cmp(start) <= 0 || join[0].Cmp(end) >= 0 {
		return zero, geom.ErrDomain
	}
	// Both centers are below the side, so every exposed portion is an upper
	// graph. The strictly ordered side intervals fix which owner is exterior;
	// the excluded second intersection precludes any further owner switch.
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
	out := UnequalCenterSideOpenDualDiskSection{LowerJoin: lower, OpeningIntervals: opening, GeneralSideOpenDualDiskSection: GeneralSideOpenDualDiskSection{Owner: owners, Join: join, XIntervals: [2][2]*big.Rat{{start, join[0]}, {join[0], end}}}}
	var loop trimvalidate.Loop
	for i, p := range points {
		z := points[(i+1)%len(points)]
		var q ExactQuadraticUV
		var e error
		if owners[i] < 0 {
			q, e = quadraticLine(p, z)
		} else {
			disk := discs[owners[i]]
			q, e = shortRationalCircleArc(p, z, [2]*big.Rat{ratInset(disk.Center.X), ratInset(disk.Center.Y)}, ratInset(disk.Radius))
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
