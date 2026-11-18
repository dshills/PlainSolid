//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// IntersectHorizontalRational intersects (origin.X+t*direction.X,
// origin.Y+t*direction.Y,z) over the exact finite closed parameter interval.
// The retained section must have an exact rational radius: all central cone
// slices and Pythagorean torus meridians qualify. Irrational section radii are
// explicitly unsupported. Cap/floor-plane coincidence is also rejected, as
// those may contain boundary intervals rather than isolated events.
// Strict native disk/axial separation proves each active bore circle is wholly
// retained; inactive bore support roots appear in TrimmedAway. At a tangent
// cone/blend seam both original faces own the same exact quadratic root.
// This reuses the axial ledger type/semantics, with at most34 candidate roots,
// 4096-bit input limits and the supplied per-root arithmetic budgets.
func (q FilletedFrustumMaterial) IntersectHorizontalRational(ctx context.Context, origin, direction [2]*big.Rat, z, min, max *big.Rat, o geom.ExactCurvePlaneEventOptions) (out FilletedAxialLine, err error) {
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
	for _, v := range []*big.Rat{origin[0], origin[1], direction[0], direction[1], z, min, max} {
		if v == nil {
			return out, geom.ErrInvalid
		}
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return out, geom.ErrConvergence
		}
	}
	if min.Cmp(max) >= 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 {
		return out, geom.ErrInvalid
	}
	if z.Cmp(q.z[0]) < 0 || z.Cmp(q.z[1]) > 0 {
		return out, nil
	}
	if z.Cmp(q.z[0]) == 0 || z.Cmp(q.z[1]) == 0 {
		return out, geom.ErrDomain
	}
	for _, c := range q.cuts {
		if c.floor >= 0 && ((!c.open[0] && z.Cmp(c.z[0]) == 0) || (!c.open[1] && z.Cmp(c.z[1]) == 0)) {
			return out, geom.ErrDomain
		}
	}
	sq := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	owners := []int{q.fillets.Lateral}
	radius := new(big.Rat).Add(q.r[0], new(big.Rat).Mul(q.slope, new(big.Rat).Sub(z, q.z[0])))
	band := -1
	if z.Cmp(q.fillets.Circles[0].Tangency[1]) <= 0 {
		band = 0
	} else if z.Cmp(q.fillets.Circles[1].Tangency[1]) >= 0 {
		band = 1
	}
	if band >= 0 {
		c := q.fillets.Circles[band]
		height2 := new(big.Rat).Sub(sq(c.Radius), sq(new(big.Rat).Sub(z, c.Center[1])))
		if height2.Sign() < 0 {
			return out, geom.ErrConvergence
		}
		n, d := new(big.Int).Sqrt(height2.Num()), new(big.Int).Sqrt(height2.Denom())
		if new(big.Int).Mul(n, n).Cmp(height2.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(height2.Denom()) != 0 {
			return out, geom.ErrDomain
		}
		radius.Add(c.Center[0], new(big.Rat).SetFrac(n, d))
		owners = []int{q.fillets.Blends[band]}
		if z.Cmp(c.Tangency[1]) == 0 {
			owners = append(owners, q.fillets.Lateral)
		}
	}
	lo, _, e := geom.NewExactRationalRoot(ctx, min, o)
	if e != nil {
		return out, e
	}
	hi, _, e := geom.NewExactRationalRoot(ctx, max, o)
	if e != nil {
		return out, e
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (int, error) {
		r, e := a.Compare(ctx, b, o)
		if e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
	}
	circle := func(center [2]*big.Rat, radius *big.Rat, faces []int, keep bool) error {
		a := new(big.Rat).Add(sq(direction[0]), sq(direction[1]))
		dx, dy := new(big.Rat).Sub(origin[0], center[0]), new(big.Rat).Sub(origin[1], center[1])
		b := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(dx, direction[0]), new(big.Rat).Mul(dy, direction[1])))
		c := new(big.Rat).Sub(new(big.Rat).Add(sq(dx), sq(dy)), sq(radius))
		disc := new(big.Rat).Sub(sq(b), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(a, c)))
		if disc.Sign() < 0 {
			return nil
		}
		middle := new(big.Rat).Neg(new(big.Rat).Quo(b, new(big.Rat).Mul(big.NewRat(2, 1), a)))
		// Radius/sqrt(a) bounds either half chord. 1+radius²/a is a strictly
		// larger rational bound, since 1+x²>x for every real x>=0.
		width := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(sq(radius), a))
		ranges := [][2]*big.Rat{{new(big.Rat).Sub(middle, width), middle}, {middle, new(big.Rat).Add(middle, width)}}
		if disc.Sign() == 0 {
			ranges = [][2]*big.Rat{{middle, middle}}
		}
		for _, interval := range ranges {
			var root *geom.ExactAlgebraicRoot
			var e error
			if disc.Sign() == 0 {
				root, _, e = geom.NewExactRationalRoot(ctx, middle, o)
			} else {
				root, e = geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{c.RatString(), b.RatString(), a.RatString()}, Lower: interval[0].RatString(), Upper: interval[1].RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o, MaxDegree: 2})
			}
			if e != nil {
				return e
			}
			left, e := compare(root, lo)
			if e != nil {
				return e
			}
			right, e := compare(root, hi)
			if e != nil {
				return e
			}
			if left < 0 || right > 0 {
				continue
			}
			if keep {
				out.Hits = append(out.Hits, FilletedAxialHit{Parameter: root, Faces: append([]int(nil), faces...)})
			} else {
				for _, face := range faces {
					out.TrimmedAway = append(out.TrimmedAway, FilletedAxialDiscard{root, face})
				}
			}
		}
		return nil
	}
	if e = circle(q.center, radius, owners, true); e != nil {
		return out, e
	}
	for _, c := range q.cuts {
		active := z.Cmp(c.z[0]) > 0 && z.Cmp(c.z[1]) < 0
		if e = circle(c.center, c.radius, []int{c.wall}, active); e != nil {
			return out, e
		}
	}
	return finishFilletedLineLedger(ctx, out, lo, hi, o, func(t *big.Rat) (FilletedFrustumMaterialPoint, error) {
		return q.ClassifyRational(ctx, [3]*big.Rat{new(big.Rat).Add(origin[0], new(big.Rat).Mul(direction[0], t)), new(big.Rat).Add(origin[1], new(big.Rat).Mul(direction[1], t)), z})
	})
}
