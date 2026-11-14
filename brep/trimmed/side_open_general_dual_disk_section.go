package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// GeneralSideOpenDualDiskSection is an exact upper-envelope cut section with
// variable circular piece counts. Owner is -1 for original stock edges, else
// the original disk index. It is deliberately separate from the two-pieces-per-
// disk section required by existing native single-wall-chart constructors.
type GeneralSideOpenDualDiskSection struct {
	Outer      []ExactQuadraticUV
	Owner      []int
	Join       [2]*big.Rat
	XIntervals [2][2]*big.Rat
}

// CertifyGeneralSideOpenDualDiskSection allows the radical crossing on either
// side of the centers. It proves strict transverse overlap, rational crossing
// height and lossless endpoints, then splits each exposed upper circle arc at
// its top when that extremum lies strictly between its endpoints. Each resulting
// piece has angle <=pi/2, avoiding tangent-half-angle poles. Both complete disks
// must strictly clear the three nonopening stock sides (a sufficient bound).
// No angular sampling, native solid assembly or circle tangency is admitted.
func CertifyGeneralSideOpenDualDiskSection(ctx context.Context, min, max geom.Vec2, discs [2]ExtrusionCircle, o trimvalidate.Options) (GeneralSideOpenDualDiskSection, error) {
	var zero GeneralSideOpenDualDiskSection
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
		if disk.Center.Y != min.Y {
			return zero, geom.ErrDomain
		}
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
	start, end := new(big.Rat).Sub(l, rl), new(big.Rat).Add(r, rr)
	topL, topR := new(big.Rat).Add(y, rl), new(big.Rat).Add(y, rr)
	if start.Cmp(ratInset(min.X)) <= 0 || end.Cmp(ratInset(max.X)) >= 0 || topL.Cmp(ratInset(max.Y)) >= 0 || topR.Cmp(ratInset(max.Y)) >= 0 {
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
	join := [2]*big.Rat{new(big.Rat).Add(l, a), new(big.Rat).Add(y, new(big.Rat).SetFrac(n, d))}
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
	out := GeneralSideOpenDualDiskSection{Owner: owners, Join: join, XIntervals: [2][2]*big.Rat{{start, join[0]}, {join[0], end}}}
	var loop trimvalidate.Loop
	for i, p := range points {
		z := points[(i+1)%len(points)]
		var q ExactQuadraticUV
		var e error
		if owners[i] < 0 {
			q, e = quadraticLine(p, z)
		} else {
			disk := discs[owners[i]]
			q, e = shortRationalCircleArc(p, z, [2]*big.Rat{ratInset(disk.Center.X), y}, ratInset(disk.Radius))
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
