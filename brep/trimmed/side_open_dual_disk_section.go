package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SideOpenDualDiskSection is the exact section of material remaining in a
// rectangle after removing two equal-radius, overlapping disks whose centers
// lie on its bottom side. Outer is CCW, with four consecutive rational
// circular pieces on the lower side and straight stock edges elsewhere.
// Owner is -1 for a stock boundary or the original input disk index.
type SideOpenDualDiskSection struct {
	Outer []ExactQuadraticUV
	Owner []int
	// Join is the sole circular crossing above the removed opening.
	Join [2]*big.Rat
	// XIntervals gives the envelope subranges for the two circle supports,
	// from left endpoint to join and join to right endpoint.
	XIntervals [2][2]*big.Rat
}

// CertifySideOpenDualDiskSection constructs the exact circular envelope
// without angle rounding. It requires a strictly transverse crossing with
// an exactly rational crossing height and exactly represented UV endpoints.
// Tangency, overlap with another stock side, unequal radii, coincident disks,
// and unsupported endpoints reject atomically.
func CertifySideOpenDualDiskSection(ctx context.Context, min, max geom.Vec2, discs [2]ExtrusionCircle, o trimvalidate.Options) (SideOpenDualDiskSection, error) {
	var zero SideOpenDualDiskSection
	if ctx == nil || !uvFinite(min) || !uvFinite(max) || min.X >= max.X || min.Y >= max.Y {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, d := range discs {
		if !uvFinite(d.Center) || !finite(d.Radius) || d.Radius <= 0 {
			return zero, geom.ErrInvalid
		}
		if d.Center.Y != min.Y {
			return zero, geom.ErrDomain
		}
	}
	if discs[0].Radius != discs[1].Radius || discs[0].Center.X == discs[1].Center.X {
		return zero, geom.ErrDomain
	}
	order := [2]int{0, 1}
	if discs[0].Center.X > discs[1].Center.X {
		order = [2]int{1, 0}
	}
	left, right := ratInset(discs[order[0]].Center.X), ratInset(discs[order[1]].Center.X)
	centerY, r := ratInset(min.Y), ratInset(discs[0].Radius)
	start, end := new(big.Rat).Sub(left, r), new(big.Rat).Add(right, r)
	top := new(big.Rat).Add(centerY, r)
	if start.Cmp(ratInset(min.X)) <= 0 || end.Cmp(ratInset(max.X)) >= 0 || top.Cmp(ratInset(max.Y)) >= 0 {
		return zero, geom.ErrDomain
	}
	half := new(big.Rat).Quo(new(big.Rat).Sub(right, left), big.NewRat(2, 1))
	if half.Cmp(r) >= 0 {
		return zero, geom.ErrDomain
	}
	height2 := new(big.Rat).Sub(new(big.Rat).Mul(r, r), new(big.Rat).Mul(half, half))
	n, d := new(big.Int).Sqrt(height2.Num()), new(big.Int).Sqrt(height2.Denom())
	if new(big.Int).Mul(n, n).Cmp(height2.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(height2.Denom()) != 0 {
		return zero, geom.ErrUnrepresentable
	}
	join := [2]*big.Rat{new(big.Rat).Add(left, half), new(big.Rat).Add(centerY, new(big.Rat).SetFrac(n, d))}
	p := [][2]*big.Rat{{ratInset(min.X), centerY}, {start, centerY}, {left, top}, join, {right, top}, {end, centerY},
		{ratInset(max.X), centerY}, {ratInset(max.X), ratInset(max.Y)}, {ratInset(min.X), ratInset(max.Y)}}
	owners := []int{-1, order[0], order[0], order[1], order[1], -1, -1, -1, -1}
	out := SideOpenDualDiskSection{Owner: owners, Join: join, XIntervals: [2][2]*big.Rat{{start, join[0]}, {join[0], end}}}
	var loop trimvalidate.Loop
	for i, a := range p {
		z := p[(i+1)%len(p)]
		var q ExactQuadraticUV
		var e error
		if owners[i] < 0 {
			q, e = quadraticLine(a, z)
		} else {
			c := [2]*big.Rat{ratInset(discs[owners[i]].Center.X), centerY}
			q, e = shortRationalCircleArc(a, z, c, r)
		}
		if e != nil {
			return zero, e
		}
		v, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
		if e != nil {
			return zero, e
		}
		loop = append(loop, v)
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

// Rotate the rational half-angle chart so t=0 is the first radius. The second
// endpoint determines rational t by tan(theta/2)=cross/(r²+dot). Substitution
// yields homogeneous degree two with strictly positive weight coefficients.
func shortRationalCircleArc(a, z, c [2]*big.Rat, r *big.Rat) (ExactQuadraticUV, error) {
	var x, y [2]*big.Rat
	for i := range x {
		x[i] = new(big.Rat).Sub(a[i], c[i])
		y[i] = new(big.Rat).Sub(z[i], c[i])
	}
	dot := new(big.Rat).Add(new(big.Rat).Mul(x[0], y[0]), new(big.Rat).Mul(x[1], y[1]))
	den := new(big.Rat).Add(new(big.Rat).Mul(r, r), dot)
	if den.Sign() <= 0 {
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(x[0], y[1]), new(big.Rat).Mul(x[1], y[0])), den)
	if t.Sign() == 0 {
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	g := new(big.Rat).Mul(t, t)
	w := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat).Add(big.NewRat(1, 1), g)}
	var h [3][3]*big.Rat
	for i := range h {
		h[i][2] = w[i]
	}
	for j := range x {
		orth := new(big.Rat).Set(x[1-j])
		if j == 0 {
			orth.Neg(orth)
		}
		h[0][j] = a[j]
		h[1][j] = new(big.Rat).Add(a[j], new(big.Rat).Mul(t, orth))
		h[2][j] = new(big.Rat).Mul(z[j], w[2])
	}
	return NewExactQuadraticUV(h)
}
