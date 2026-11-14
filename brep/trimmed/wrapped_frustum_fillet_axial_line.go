package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedAxialHit has an immutable exact ORIGINAL t identity. Kind is entry,
// exit, tangent or endpoint; at a finite endpoint one-sided status is unknown.
type FilletedAxialHit struct {
	Parameter               *geom.ExactAlgebraicRoot
	Faces                   []int
	Kind                    string
	Before, After           geom.BorePointLocation
	BeforeKnown, AfterKnown bool
}
type FilletedAxialDiscard struct {
	Parameter *geom.ExactAlgebraicRoot
	Face      int
}
type FilletedAxialLine struct {
	Hits        []FilletedAxialHit
	TrimmedAway []FilletedAxialDiscard
}

// IntersectAxialRational intersects (x,y,z0+dz*t) on closed [min,max]. This
// bounded subset requires rational sqrt((x-cx)^2+(y-cy)^2); irrational radial
// distances and lines coincident with any bore cylinder are rejected. The
// toroidal height roots may be irrational and retain exact polynomial proof.
// Candidate positive-radius cone, meridian-circle and cap/floor plane
// contacts in the finite interval become native hits or explicit trimmed-away
// evidence (no claim about the opposite nappe of an extended double cone). Cap/floor
// plane contacts are filtered by exact point membership and original face ID.
// At most71 original contacts and73 interval cells are considered. Input
// rationals are capped at4096 bits; each root import/map/order/refinement uses
// the supplied independent proof budget. Any unresolved operation is atomic.
func (q FilletedFrustumMaterial) IntersectAxialRational(ctx context.Context, x, y, z0, dz, min, max *big.Rat, o geom.ExactCurvePlaneEventOptions) (out FilletedAxialLine, err error) {
	defer func() {
		if err != nil {
			out = FilletedAxialLine{}
		}
	}()
	if ctx == nil || q.slope == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, v := range []*big.Rat{x, y, z0, dz, min, max} {
		if v == nil {
			return out, geom.ErrInvalid
		}
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return out, geom.ErrConvergence
		}
	}
	if dz.Sign() == 0 || min.Cmp(max) >= 0 {
		return out, geom.ErrInvalid
	}
	sq := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	radial2 := new(big.Rat).Add(sq(new(big.Rat).Sub(x, q.center[0])), sq(new(big.Rat).Sub(y, q.center[1])))
	rootInt := func(v *big.Int) (*big.Int, bool) {
		r := new(big.Int).Sqrt(v)
		return r, new(big.Int).Mul(r, r).Cmp(v) == 0
	}
	n, ok := rootInt(radial2.Num())
	if !ok {
		return out, geom.ErrDomain
	}
	d, ok := rootInt(radial2.Denom())
	if !ok {
		return out, geom.ErrDomain
	}
	rho := new(big.Rat).SetFrac(n, d)
	for _, c := range q.cuts {
		r2 := new(big.Rat).Add(sq(new(big.Rat).Sub(x, c.center[0])), sq(new(big.Rat).Sub(y, c.center[1])))
		if r2.Cmp(sq(c.radius)) == 0 {
			return out, geom.ErrDomain
		}
	}
	rational := func(v *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		r, _, e := geom.NewExactRationalRoot(ctx, v, o)
		return r, e
	}
	lo, e := rational(min)
	if e != nil {
		return out, e
	}
	hi, e := rational(max)
	if e != nil {
		return out, e
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (int, error) {
		v, e := a.Compare(ctx, b, o)
		if e != nil {
			return 0, e
		}
		if !v.Proven {
			return 0, geom.ErrConvergence
		}
		return v.Order, nil
	}
	inv := new(big.Rat).Inv(dz)
	offset := new(big.Rat).Neg(new(big.Rat).Mul(z0, inv))
	add := func(z *geom.ExactAlgebraicRoot, face int, keep bool) error {
		t, _, e := z.AffineMap(ctx, offset, inv, o)
		if e != nil {
			return e
		}
		a, e := compare(t, lo)
		if e != nil {
			return e
		}
		b, e := compare(t, hi)
		if e != nil {
			return e
		}
		if a < 0 || b > 0 {
			return nil
		}
		if keep {
			out.Hits = append(out.Hits, FilletedAxialHit{Parameter: t, Faces: []int{face}})
		} else {
			out.TrimmedAway = append(out.TrimmedAway, FilletedAxialDiscard{t, face})
		}
		return nil
	}
	// Plane roots are rational. Native ownership distinguishes a retained disk
	// from a trimmed-away infinite support even where another face is boundary.
	plane := func(z *big.Rat, face int) error {
		p, e := q.ClassifyRational(ctx, [3]*big.Rat{x, y, z})
		if e != nil {
			return e
		}
		keep := false
		for _, id := range p.Faces {
			keep = keep || id == face
		}
		r, e := rational(z)
		if e != nil {
			return e
		}
		return add(r, face, keep)
	}
	for end, z := range q.z {
		if e = plane(z, q.fillets.Caps[end]); e != nil {
			return out, e
		}
	}
	for _, c := range q.cuts {
		if c.floor >= 0 {
			z := c.z[0]
			if c.open[0] {
				z = c.z[1]
			}
			if e = plane(z, c.floor); e != nil {
				return out, e
			}
		}
	}
	coneZ := new(big.Rat).Add(q.z[0], new(big.Rat).Quo(new(big.Rat).Sub(rho, q.r[0]), q.slope))
	cr, e := rational(coneZ)
	if e != nil {
		return out, e
	}
	keep := coneZ.Cmp(q.fillets.Circles[0].Tangency[1]) >= 0 && coneZ.Cmp(q.fillets.Circles[1].Tangency[1]) <= 0
	if e = add(cr, q.fillets.Lateral, keep); e != nil {
		return out, e
	}
	for end, c := range q.fillets.Circles {
		// The full torus has two radial branches. Both height solutions are
		// recorded, but only rho>=M on the authored meridian height is retained.
		delta := new(big.Rat).Sub(rho, c.Center[0])
		height2 := new(big.Rat).Sub(sq(c.Radius), sq(delta))
		if height2.Sign() < 0 {
			continue
		}
		ranges := [][2]*big.Rat{{new(big.Rat).Sub(c.Center[1], c.Radius), c.Center[1]}, {c.Center[1], new(big.Rat).Add(c.Center[1], c.Radius)}}
		if height2.Sign() == 0 {
			ranges = [][2]*big.Rat{{c.Center[1], c.Center[1]}}
		}
		for _, bounds := range ranges {
			var root *geom.ExactAlgebraicRoot
			if height2.Sign() == 0 {
				root, e = rational(c.Center[1])
			} else {
				constant := new(big.Rat).Sub(sq(c.Center[1]), height2)
				linear := new(big.Rat).Mul(big.NewRat(-2, 1), c.Center[1])
				root, e = geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{constant.RatString(), linear.RatString(), "1"}, Lower: bounds[0].RatString(), Upper: bounds[1].RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o, MaxDegree: 2})
			}
			if e != nil {
				return out, e
			}
			a, b := q.z[end], c.Tangency[1]
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			ra, e := rational(a)
			if e != nil {
				return out, e
			}
			rb, e := rational(b)
			if e != nil {
				return out, e
			}
			left, e := compare(root, ra)
			if e != nil {
				return out, e
			}
			right, e := compare(root, rb)
			if e != nil {
				return out, e
			}
			if e = add(root, q.fillets.Blends[end], delta.Sign() >= 0 && left >= 0 && right <= 0); e != nil {
				return out, e
			}
		}
	}
	// Exact insertion ordering merges only proven equal algebraic parameters.
	for i := 1; i < len(out.Hits); i++ {
		for j := i; j > 0; j-- {
			order, e := compare(out.Hits[j-1].Parameter, out.Hits[j].Parameter)
			if e != nil {
				return out, e
			}
			if order <= 0 {
				break
			}
			out.Hits[j-1], out.Hits[j] = out.Hits[j], out.Hits[j-1]
		}
	}
	merged := []FilletedAxialHit{}
	for _, h := range out.Hits {
		if len(merged) > 0 {
			order, e := compare(merged[len(merged)-1].Parameter, h.Parameter)
			if e != nil {
				return out, e
			}
			if order == 0 {
				merged[len(merged)-1].Faces = append(merged[len(merged)-1].Faces, h.Faces...)
				continue
			}
		}
		merged = append(merged, h)
	}
	out.Hits = merged
	between := func(a, b *geom.ExactAlgebraicRoot) (geom.BorePointLocation, error) {
		r, e := a.Compare(ctx, b, o)
		if e != nil {
			return geom.BoreOutside, e
		}
		if !r.Proven || r.Order >= 0 {
			return geom.BoreOutside, geom.ErrConvergence
		}
		left, right := r.Intervals[0][1], r.Intervals[1][0]
		if left == nil || right == nil || left.Cmp(right) >= 0 {
			return geom.BoreOutside, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
		z := new(big.Rat).Add(z0, new(big.Rat).Mul(dz, t))
		p, e := q.ClassifyRational(ctx, [3]*big.Rat{x, y, z})
		if e != nil {
			return geom.BoreOutside, e
		}
		if p.Location == geom.BoreBoundary {
			return geom.BoreOutside, geom.ErrConvergence
		}
		return p.Location, nil
	}
	for i := range out.Hits {
		h := &out.Hits[i]
		sort.Ints(h.Faces)
		a, b := lo, hi
		if i > 0 {
			a = out.Hits[i-1].Parameter
		}
		if i+1 < len(out.Hits) {
			b = out.Hits[i+1].Parameter
		}
		order, e := compare(a, h.Parameter)
		if e != nil {
			return out, e
		}
		if order < 0 {
			h.Before, e = between(a, h.Parameter)
			if e != nil {
				return out, e
			}
			h.BeforeKnown = true
		}
		order, e = compare(h.Parameter, b)
		if e != nil {
			return out, e
		}
		if order < 0 {
			h.After, e = between(h.Parameter, b)
			if e != nil {
				return out, e
			}
			h.AfterKnown = true
		}
		h.Kind = "endpoint"
		if h.BeforeKnown && h.AfterKnown {
			h.Kind = "tangent"
			if h.Before == geom.BoreOutside && h.After == geom.BoreInside {
				h.Kind = "entry"
			}
			if h.Before == geom.BoreInside && h.After == geom.BoreOutside {
				h.Kind = "exit"
			}
		}
	}
	for i := 1; i < len(out.TrimmedAway); i++ {
		for j := i; j > 0; j-- {
			order, e := compare(out.TrimmedAway[j-1].Parameter, out.TrimmedAway[j].Parameter)
			if e != nil {
				return out, e
			}
			if order < 0 || order == 0 && out.TrimmedAway[j-1].Face <= out.TrimmedAway[j].Face {
				break
			}
			out.TrimmedAway[j-1], out.TrimmedAway[j] = out.TrimmedAway[j], out.TrimmedAway[j-1]
		}
	}
	return out, nil
}
