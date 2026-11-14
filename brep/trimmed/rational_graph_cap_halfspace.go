package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// intersectCapHalfspace clips one proved outer section ring with two regular
// boundary crossings. It retains exact Bezier pieces and inserts one chord.
// The returned parameters are indexed by original section piece; the caller
// can split the companion source trim at the same native parameter.
// Complete holes strictly on one side survive or are omitted as a whole.
// Tangencies, contacts on a hole or original vertex, and multiple runs fail.
func (r *RationalGraphShellRegions) intersectCapHalfspace(ctx context.Context, other polynomialPlaneEquation, keep int, o RationalGraphShellRegionOptions) (_ *RationalGraphShellRegions, out [2]capBoundaryCut, err error) {
	defer func() {
		if err != nil {
			out = [2]capBoundaryCut{}
		}
	}()
	var cuts [2]capBoundaryCut
	if ctx == nil || r == nil || r.certificate == nil || keep != -1 && keep != 1 {
		return nil, cuts, geom.ErrInvalid
	}
	if len(r.material) != 1 || len(r.wires) != 1+len(r.material[0].Holes) {
		return nil, cuts, geom.ErrConvergence
	}
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return nil, cuts, e
	}
	wire := r.wires[r.material[0].Outer]
	if len(wire) >= opts.MaxEdges {
		return nil, cuts, geom.ErrConvergence
	}
	limits := geom.SurfacePatchOptions{MaxOperations: opts.MaxWork, MaxCoefficientBits: opts.MaxBits}
	work, subdivisions := opts.MaxWork, opts.MaxSubdivisions
	rootOptions := func() geom.ExactCurvePlaneEventOptions {
		return geom.ExactCurvePlaneEventOptions{MaxOperations: work, MaxCoefficientBits: opts.MaxBits, MaxSubdivisions: max(1, subdivisions)}
	}
	charge := func(n int) error {
		work -= n
		if work <= 0 {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	form := r.capHalfspaceForm(other)
	for _, a := range form {
		if a.Num().BitLen() > opts.MaxBits || a.Denom().BitLen() > opts.MaxBits {
			return nil, cuts, geom.ErrConvergence
		}
	}
	if form[0].Sign() == 0 && form[1].Sign() == 0 {
		return nil, cuts, geom.ErrConvergence
	}
	var kept []projectedGraphSectionPiece
	var order []int
	n := 0
	sign := func(h [3]*big.Rat) int {
		v := new(big.Rat)
		for i, a := range form {
			v.Add(v, new(big.Rat).Mul(a, h[i]))
		}
		return v.Sign() * other.orientation * keep
	}
	var holes [][]projectedGraphSectionPiece
	for _, id := range r.material[0].Holes {
		hole, side := r.wires[id], 0
		for _, p := range hole {
			for _, h := range p.control {
				if e := charge(1); e != nil {
					return nil, cuts, e
				}
				q := sign(h)
				if q == 0 || side != 0 && q != side {
					return nil, cuts, geom.ErrConvergence
				}
				side = q
			}
		}
		if side > 0 {
			holes = append(holes, hole)
		}
	}
	for i, p := range wire {
		if e := charge(1); e != nil {
			return nil, cuts, e
		}
		a, b := sign(p.control[0]), sign(p.control[len(p.control)-1])
		if a == 0 || b == 0 {
			return nil, cuts, geom.ErrConvergence
		}
		sameSign := true
		for _, h := range p.control[1:] {
			sameSign = sameSign && sign(h) == a
		}
		if !sameSign {
			// A complete exact root count catches a missed touch or an
			// even number of crossings despite equally signed endpoints.
			controls := make([][4]*big.Rat, len(p.control))
			for j, h := range p.control {
				controls[j] = [4]*big.Rat{h[0], h[1], new(big.Rat), h[2]}
			}
			limits.MaxOperations = work
			curve, e := geom.NewExactHomogeneousBezier(ctx, controls, limits)
			if e != nil {
				return nil, cuts, e
			}
			cell, e := geom.NewExactComposedBezierCell(ctx, curve, limits)
			if e != nil {
				return nil, cuts, e
			}
			raw := [4]*big.Rat{form[0], form[1], nil, form[2]}
			roots, e := cell.IntersectRawPlane(ctx, raw, rootOptions())
			if e != nil {
				return nil, cuts, e
			}
			if e = charge(roots.Operations); e != nil {
				return nil, cuts, e
			}
			subdivisions -= roots.Subdivisions
			if subdivisions < 0 {
				return nil, cuts, geom.ErrConvergence
			}
			if !roots.Complete || roots.Coincident || a == b && len(roots.Events) != 0 || a != b && len(roots.Events) != 1 {
				return nil, cuts, geom.ErrConvergence
			}
			if a != b {
				event := roots.Events[0]
				if n == 2 || event.EndpointMask() != 0 || event.Multiplicity() != 1 {
					return nil, cuts, geom.ErrConvergence
				}
				entry, e := rationalCapIntersectionParameter(ctx, event, rootOptions(), &subdivisions, charge)
				if e != nil {
					return nil, cuts, e
				}
				left, right, e := splitCapBezier(p.control, entry, opts.MaxBits)
				if e != nil {
					return nil, cuts, e
				}
				if sign(right[0]) != 0 {
					return nil, cuts, geom.ErrConvergence
				}
				cuts[n] = capBoundaryCut{p.source, entry, a > 0, r.capPoint(right[0])}
				n++
				if a > 0 {
					p.control = left
				} else {
					p.control = right
				}
			}
		}
		if a > 0 || b > 0 {
			kept = append(kept, p)
			order = append(order, i)
		}
	}
	if n != 2 || cuts[0].KeepBefore == cuts[1].KeepBefore || len(kept) == 0 {
		return nil, [2]capBoundaryCut{}, geom.ErrConvergence
	}
	first := -1
	for i, p := range kept {
		if p.source == cuts[0].Source && !cuts[0].KeepBefore || p.source == cuts[1].Source && !cuts[1].KeepBefore {
			first = i
		}
	}
	if first < 0 {
		return nil, [2]capBoundaryCut{}, geom.ErrConvergence
	}
	path := make([]projectedGraphSectionPiece, 0, len(kept)+1)
	for i := range kept {
		j := (first + i) % len(kept)
		if i > 0 && (order[(first+i-1)%len(kept)]+1)%len(wire) != order[j] {
			return nil, [2]capBoundaryCut{}, geom.ErrConvergence
		}
		path = append(path, kept[j])
	}
	a, z := path[0].control[0], path[len(path)-1].control
	b := z[len(z)-1]
	// Normalize both endpoints so the two cap charts independently
	// produce the same affine parameter along the common line.
	h0, h1 := [3]*big.Rat{nil, nil, big.NewRat(1, 1)}, [3]*big.Rat{nil, nil, big.NewRat(1, 1)}
	mid := [3]*big.Rat{nil, nil, big.NewRat(1, 1)}
	for i := range 2 {
		h0[i], h1[i] = new(big.Rat).Quo(b[i], b[2]), new(big.Rat).Quo(a[i], a[2])
		mid[i] = new(big.Rat).Quo(new(big.Rat).Add(h0[i], h1[i]), big.NewRat(2, 1))
	}
	path = append(path, projectedGraphSectionPiece{source: RationalGraphCapEdgeSource{Wire: -1, Branch: -1, Piece: -1, Face: -1}, control: [][3]*big.Rat{h0, mid, h1}})
	wires := append([][]projectedGraphSectionPiece{path}, holes...)
	rings := make([]trimvalidate.Loop, len(wires))
	for i, wire := range wires {
		for _, p := range wire {
			q, e := trimvalidate.ExactBezier(p.control)
			if e != nil {
				return nil, [2]capBoundaryCut{}, e
			}
			rings[i] = append(rings[i], q)
		}
	}
	if subdivisions == 0 {
		return nil, [2]capBoundaryCut{}, geom.ErrConvergence
	}
	opts.MaxWork, opts.MaxSubdivisions = work, subdivisions
	keptRegion, e := certifyProjectedRationalGraphRegions(ctx, wires, rings, r.plane, opts)
	if e != nil {
		return nil, [2]capBoundaryCut{}, e
	}
	return keptRegion, cuts, nil
}

func rationalCapIntersectionParameter(ctx context.Context, r *geom.ExactComposedPlaneEvent, o geom.ExactCurvePlaneEventOptions, subdivisions *int, charge func(int) error) (*big.Rat, error) {
	interval := r.Interval()
	if interval.Lower.Cmp(interval.Upper) == 0 {
		return interval.Lower, nil
	}
	// Bisection can find dyadic roots, but a linear polynomial also gives
	// a rational root with any denominator. The original unique-root proof
	// and later exact homogeneous substitution both recheck that value.
	recipe, e := r.RootRecipe(ctx, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o, MaxDegree: 4})
	if e != nil {
		return nil, e
	}
	if len(recipe.Coefficients) == 2 {
		a, aok := new(big.Rat).SetString(recipe.Coefficients[0])
		b, bok := new(big.Rat).SetString(recipe.Coefficients[1])
		if !aok || !bok || b.Sign() == 0 {
			return nil, geom.ErrConvergence
		}
		at := new(big.Rat).Quo(a.Neg(a), b)
		if at.Cmp(interval.Lower) < 0 || at.Cmp(interval.Upper) > 0 {
			return nil, geom.ErrConvergence
		}
		return at, nil
	}
	if len(recipe.Coefficients) == 3 {
		var p [3]*big.Rat
		for i, s := range recipe.Coefficients {
			var ok bool
			p[i], ok = new(big.Rat).SetString(s)
			if !ok {
				return nil, geom.ErrConvergence
			}
		}
		a, b, c := p[2], p[1], p[0]
		discriminant := new(big.Rat).Sub(new(big.Rat).Mul(b, b), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(a, c)))
		if a.Sign() == 0 || discriminant.Sign() <= 0 || discriminant.Num().BitLen() > o.MaxCoefficientBits || discriminant.Denom().BitLen() > o.MaxCoefficientBits {
			return nil, geom.ErrConvergence
		}
		n, d := new(big.Int).Sqrt(discriminant.Num()), new(big.Int).Sqrt(discriminant.Denom())
		if new(big.Int).Mul(n, n).Cmp(discriminant.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(discriminant.Denom()) != 0 {
			return nil, geom.ErrConvergence
		}
		shift, denom := new(big.Rat).SetFrac(n, d), new(big.Rat).Mul(a, big.NewRat(2, 1))
		negB := new(big.Rat).Neg(b)
		for _, shift := range []*big.Rat{shift, new(big.Rat).Neg(shift)} {
			at := new(big.Rat).Quo(new(big.Rat).Add(negB, shift), denom)
			if at.Cmp(interval.Lower) >= 0 && at.Cmp(interval.Upper) <= 0 {
				return at, nil
			}
		}
		return nil, geom.ErrConvergence
	}
	q, e := r.Root().Enclose(ctx, new(big.Rat), o)
	if e != nil {
		return nil, e
	}
	if !q.Converged || q.Interval.Lower.Cmp(q.Interval.Upper) != 0 {
		return nil, geom.ErrConvergence
	}
	if e = charge(q.Operations); e != nil {
		return nil, e
	}
	*subdivisions -= q.Subdivisions
	if *subdivisions < 0 {
		return nil, geom.ErrConvergence
	}
	return q.Interval.Lower, nil
}

type capBoundaryCut struct {
	Source     RationalGraphCapEdgeSource
	Parameter  *big.Rat
	KeepBefore bool
	Point      [3]*big.Rat
}

func (r *RationalGraphShellRegions) capPoint(h [3]*big.Rat) [3]*big.Rat {
	var p [3]*big.Rat
	p[r.plane.dropped] = new(big.Rat).Set(r.plane.offset)
	for i, a := range orientedRationalGraphProjection(r.plane) {
		p[a] = new(big.Rat).Quo(h[i], h[2])
		p[r.plane.dropped].Sub(p[r.plane.dropped], new(big.Rat).Mul(r.plane.normal[a], p[a]))
	}
	return p
}

func splitCapBezier(control [][3]*big.Rat, at *big.Rat, bits int) (left, right [][3]*big.Rat, err error) {
	if at == nil || at.Sign() <= 0 || at.Cmp(big.NewRat(1, 1)) >= 0 || len(control) < 2 || len(control) > 5 {
		return nil, nil, geom.ErrConvergence
	}
	n := len(control)
	left, right = make([][3]*big.Rat, n), make([][3]*big.Rat, n)
	row := append([][3]*big.Rat(nil), control...)
	left[0], right[n-1] = row[0], row[n-1]
	for i := 1; i < n; i++ {
		for j := 0; j < n-i; j++ {
			for k := range row[j] {
				v := new(big.Rat).Sub(row[j+1][k], row[j][k])
				v.Add(row[j][k], v.Mul(v, at))
				if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
					return nil, nil, geom.ErrConvergence
				}
				row[j][k] = v
			}
		}
		left[i], right[n-1-i] = row[0], row[n-1-i]
	}
	return left, right, nil
}
