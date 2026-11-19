//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedObliqueLine retains original-t algebraic roots and native face IDs.
// RootOperations cumulatively charges isolation, exact trim/branch signs and
// ordering/refinement. Fixed-degree coefficient construction and bounded
// rational material classification have separate structural/input bit bounds.
type FilletedObliqueLine struct {
	FilletedAxialLine
	RootOperations int
}

// IntersectLineRational queries a finite rational 3D line on the immutable
// proved conical/toroidal stock and separated finite cuts. Original support
// quadratics/quartics are isolated completely. Cone positive-radius and torus
// outer-branch guards plus all exact axial trims are checked at each immutable
// root. Cap/floor plane roots use exact rational native point ownership.
// Identically coincident support polynomials reject (even if a later trim
// might remove them); no interval-boundary behavior is inferred. MaxOperations
// is cumulative across the entire root ledger (default2M), not per support.
// At most2 cap planes+16 floors+16 cylinders+2 quartics+1 cone are examined.
// Input rationals are capped at4096 bits. No partial result escapes failure.
func (q FilletedFrustumMaterial) IntersectLineRational(ctx context.Context, origin, direction [3]*big.Rat, min, max *big.Rat, o geom.ExactCurvePlaneEventOptions) (out FilletedObliqueLine, err error) {
	defer func() {
		if err != nil {
			out = FilletedObliqueLine{}
		}
	}()
	if ctx == nil || q.slope == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, v := range []*big.Rat{origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], min, max} {
		if v == nil {
			return out, geom.ErrInvalid
		}
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return out, geom.ErrConvergence
		}
	}
	if min.Cmp(max) >= 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return out, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxOperations < 1 {
		return out, geom.ErrInvalid
	}
	remaining := o.MaxOperations
	options := func() (geom.ExactCurvePlaneEventOptions, error) {
		v := o
		if remaining < 1 {
			return v, geom.ErrConvergence
		}
		v.MaxOperations = remaining
		return v, nil
	}
	addPoly := func(a, b []*big.Rat, sign int64) []*big.Rat {
		n := len(a)
		if len(b) > n {
			n = len(b)
		}
		p := make([]*big.Rat, n)
		for i := range p {
			p[i] = new(big.Rat)
			if i < len(a) {
				p[i].Set(a[i])
			}
			if i < len(b) {
				p[i].Add(p[i], new(big.Rat).Mul(big.NewRat(sign, 1), b[i]))
			}
		}
		return p
	}
	product := func(a, b []*big.Rat) []*big.Rat {
		p := make([]*big.Rat, len(a)+len(b)-1)
		for i := range p {
			p[i] = new(big.Rat)
		}
		for i, x := range a {
			for j, y := range b {
				p[i+j].Add(p[i+j], new(big.Rat).Mul(x, y))
			}
		}
		return p
	}
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	radial := func(center [2]*big.Rat) []*big.Rat {
		a := []*big.Rat{new(big.Rat).Sub(origin[0], center[0]), direction[0]}
		b := []*big.Rat{new(big.Rat).Sub(origin[1], center[1]), direction[1]}
		return addPoly(product(a, a), product(b, b), 1)
	}
	zminus := func(z *big.Rat) []*big.Rat { return []*big.Rat{new(big.Rat).Sub(origin[2], z), direction[2]} }
	sign := func(root *geom.ExactAlgebraicRoot, p []*big.Rat) (int, error) {
		options, e := options()
		if e != nil {
			return 0, e
		}
		v, e := root.SignPolynomial(ctx, p, options)
		remaining -= v.Operations
		return v.Sign, e
	}
	inZ := func(root *geom.ExactAlgebraicRoot, lo, hi *big.Rat) (bool, error) {
		a, e := sign(root, zminus(lo))
		if e != nil {
			return false, e
		}
		b, e := sign(root, zminus(hi))
		return a >= 0 && b <= 0, e
	}
	generate := func(poly []*big.Rat, face int, filter func(*geom.ExactAlgebraicRoot) (bool, error)) error {
		options, e := options()
		if e != nil {
			return e
		}
		roots, e := geom.IsolateExactPolynomialRoots(ctx, poly, min, max, options)
		remaining -= roots.Operations
		if e != nil {
			return e
		}
		for _, v := range roots.Roots {
			keep, e := filter(v.Root)
			if e != nil {
				return e
			}
			if keep {
				out.Hits = append(out.Hits, FilletedAxialHit{Parameter: v.Root, Faces: []int{face}})
			} else {
				out.TrimmedAway = append(out.TrimmedAway, FilletedAxialDiscard{Parameter: v.Root, Face: face})
			}
		}
		return nil
	}
	plane := func(z *big.Rat, owner int) error {
		return generate(zminus(z), owner, func(_ *geom.ExactAlgebraicRoot) (bool, error) {
			if direction[2].Sign() == 0 {
				return false, geom.ErrDomain
			}
			t := new(big.Rat).Quo(new(big.Rat).Sub(z, origin[2]), direction[2])
			var p [3]*big.Rat
			for i := range p {
				p[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(direction[i], t))
			}
			v, e := q.ClassifyRational(ctx, p)
			if e != nil {
				return false, e
			}
			for _, id := range v.Faces {
				if id == owner {
					return true, nil
				}
			}
			return false, nil
		})
	}
	for end, z := range q.z {
		if e := plane(z, q.fillets.Caps[end]); e != nil {
			return out, e
		}
	}
	for _, c := range q.cuts {
		if c.floor >= 0 {
			z := c.z[0]
			if c.open[0] {
				z = c.z[1]
			}
			if e := plane(z, c.floor); e != nil {
				return out, e
			}
		}
	}
	rho2 := radial(q.center)
	coneRadius := []*big.Rat{new(big.Rat).Add(q.r[0], new(big.Rat).Mul(q.slope, new(big.Rat).Sub(origin[2], q.z[0]))), new(big.Rat).Mul(q.slope, direction[2])}
	if e := generate(addPoly(rho2, product(coneRadius, coneRadius), -1), q.fillets.Lateral, func(root *geom.ExactAlgebraicRoot) (bool, error) {
		positive, e := sign(root, coneRadius)
		if e != nil {
			return false, e
		}
		inside, e := inZ(root, q.fillets.Circles[0].Tangency[1], q.fillets.Circles[1].Tangency[1])
		return positive > 0 && inside, e
	}); e != nil {
		return out, e
	}
	for end, c := range q.fillets.Circles {
		height := zminus(c.Center[1])
		a := addPoly(rho2, product(height, height), 1)
		a[0].Add(a[0], new(big.Rat).Sub(square(c.Center[0]), square(c.Radius)))
		factor := new(big.Rat).Mul(big.NewRat(4, 1), square(c.Center[0]))
		b := make([]*big.Rat, len(rho2))
		for i, v := range rho2 {
			b[i] = new(big.Rat).Mul(v, factor)
		}
		poly := addPoly(product(a, a), b, -1)
		guard := addPoly(rho2, []*big.Rat{square(c.Center[0])}, -1)
		lo, hi := q.z[end], c.Tangency[1]
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if e := generate(poly, q.fillets.Blends[end], func(root *geom.ExactAlgebraicRoot) (bool, error) {
			outer, e := sign(root, guard)
			if e != nil {
				return false, e
			}
			inside, e := inZ(root, lo, hi)
			return outer >= 0 && inside, e
		}); e != nil {
			return out, e
		}
	}
	for _, c := range q.cuts {
		poly := radial(c.center)
		poly[0].Sub(poly[0], square(c.radius))
		if e := generate(poly, c.wall, func(root *geom.ExactAlgebraicRoot) (bool, error) { return inZ(root, c.z[0], c.z[1]) }); e != nil {
			return out, e
		}
	}
	rational := func(x *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		options, e := options()
		if e != nil {
			return nil, e
		}
		root, work, e := geom.NewExactRationalRoot(ctx, x, options)
		remaining -= work
		return root, e
	}
	lo, e := rational(min)
	if e != nil {
		return out, e
	}
	hi, e := rational(max)
	if e != nil {
		return out, e
	}
	out.FilletedAxialLine, e = finishFilletedLineLedgerBudget(ctx, out.FilletedAxialLine, lo, hi, o, func(t *big.Rat) (FilletedFrustumMaterialPoint, error) {
		var p [3]*big.Rat
		for i := range p {
			p[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(direction[i], t))
		}
		return q.ClassifyRational(ctx, p)
	}, &remaining)
	if e != nil {
		return out, e
	}
	out.RootOperations = o.MaxOperations - remaining
	return out, nil
}
