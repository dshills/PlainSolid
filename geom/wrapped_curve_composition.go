package geom

import (
	"context"
	"math/big"
	"sort"
)

// WrappedCurvePiece retains original normalized parameters, the original
// lifted UV subcurve, exact model curve and selected fundamental knot cell.
// PeriodShift is in source periods; UV is never silently reduced.
type WrappedCurvePiece struct {
	Start, End  *big.Rat
	UV, Curve   *ExactSplineCurve
	Cell        ParameterRectangle
	PeriodShift [2]*big.Int
}
type WrappedCurveComposition struct {
	Pieces               []WrappedCurvePiece
	Complete, Continuous bool
	ExactOperations      int
}

// ComposeParameterCurvePieces composes one positive rational linear/quadratic
// UV Bezier with the original wrapped support. Seam roots must be rational;
// a nonsquare quadratic discriminant remains unsupported (even if its roots
// may lie outside the domain). Fundamental knot roots use the existing bounded
// exact composition proof. Model joins are compared exactly. No float cut or
// representative point establishes identity. Any failure returns a zero result.
func (w *WrappedSurfacePatch) ComposeParameterCurvePieces(ctx context.Context, curve *ExactSplineCurve, o SurfaceCurveCompositionOptions) (WrappedCurveComposition, error) {
	var zero WrappedCurveComposition
	if ctx == nil || w == nil || w.patch == nil || curve == nil || curve.patch == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return zero, e
	}
	o.SurfacePatchOptions = opts
	if o.MaxPieces == 0 {
		o.MaxPieces = 4096
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxHullDepth == 0 {
		o.MaxHullDepth = 32
	}
	if o.MaxPieces < 1 || o.MaxPieces > 65536 || o.MaxRootSubdivisions < 1 || o.MaxHullDepth < 1 || o.MaxHullDepth > 64 {
		return zero, ErrInvalid
	}
	c := curve.patch
	lo, hi := curve.Domain()
	if lo != 0 || hi != 1 || c.countU != c.degreeU+1 || c.degreeU < 1 || c.degreeU > 2 || len(c.net) > opts.MaxPoles || len(w.patch.net) > opts.MaxPoles {
		return zero, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	net := make([]exactHomogeneous, c.countU)
	for i := range net {
		net[i] = c.net[i*c.countV]
		if net[i][2].Sign() != 0 || net[i][3].Sign() <= 0 {
			return zero, ErrInvalid
		}
		for _, q := range net[i] {
			if e := budget.check(q); e != nil {
				return zero, e
			}
		}
	}
	floor := func(q *big.Rat) *big.Int {
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if r.Sign() < 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	seen := map[string]bool{"0": true, "1": true}
	domain := w.Domain()
	mins := [2]float64{domain.UMin, domain.VMin}
	for axis, period := range w.period {
		if period == nil {
			continue
		}
		var lower, upper *big.Rat
		for _, h := range net {
			q := new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(q); e != nil {
				return zero, e
			}
			if lower == nil || q.Cmp(lower) < 0 {
				lower = q
			}
			if upper == nil || q.Cmp(upper) > 0 {
				upper = q
			}
		}
		origin := new(big.Rat).SetFloat64(mins[axis])
		first := floor(new(big.Rat).Quo(new(big.Rat).Sub(lower, origin), period))
		last := floor(new(big.Rat).Quo(new(big.Rat).Sub(upper, origin), period))
		n := new(big.Int).Sub(last, first)
		if !n.IsInt64() || n.Int64() > int64(o.MaxPieces) {
			return zero, ErrConvergence
		}
		for k := new(big.Int).Set(first); k.Cmp(last) <= 0; k.Add(k, big.NewInt(1)) {
			if e := ctx.Err(); e != nil {
				return zero, e
			}
			seam := new(big.Rat).Add(origin, new(big.Rat).Mul(new(big.Rat).SetInt(k), period))
			if e := budget.check(seam); e != nil {
				return zero, e
			}
			if seam.Cmp(lower) < 0 || seam.Cmp(upper) > 0 {
				continue
			}
			values := make([]*big.Rat, len(net))
			for i, h := range net {
				values[i] = new(big.Rat).Sub(h[axis], new(big.Rat).Mul(seam, h[3]))
				if e := budget.check(values[i]); e != nil {
					return zero, e
				}
			}
			p, e := bernsteinToPower(values, budget)
			if e != nil {
				return zero, e
			}
			var roots []*big.Rat
			switch len(p) {
			case 0, 1:
			case 2:
				roots = append(roots, new(big.Rat).Quo(new(big.Rat).Neg(p[0]), p[1]))
			case 3:
				disc := new(big.Rat).Sub(new(big.Rat).Mul(p[1], p[1]), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(p[0], p[2])))
				if e := budget.check(disc); e != nil {
					return zero, e
				}
				if disc.Sign() < 0 {
					continue
				}
				sn, sd := new(big.Int).Sqrt(disc.Num()), new(big.Int).Sqrt(disc.Denom())
				if new(big.Int).Mul(sn, sn).Cmp(disc.Num()) != 0 || new(big.Int).Mul(sd, sd).Cmp(disc.Denom()) != 0 {
					return zero, ErrConvergence
				}
				sqrt := new(big.Rat).SetFrac(sn, sd)
				den := new(big.Rat).Mul(big.NewRat(2, 1), p[2])
				for _, sign := range []int64{-1, 1} {
					roots = append(roots, new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Neg(p[1]), new(big.Rat).Mul(big.NewRat(sign, 1), sqrt)), den))
				}
			default:
				return zero, ErrConvergence
			}
			for _, root := range roots {
				if e := budget.check(root); e != nil {
					return zero, e
				}
				if root.Sign() > 0 && root.Cmp(big.NewRat(1, 1)) < 0 && !seen[root.RatString()] {
					if len(cuts)-1 >= o.MaxPieces {
						return zero, ErrConvergence
					}
					seen[root.RatString()] = true
					cuts = append(cuts, root)
				}
			}
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	r := WrappedCurveComposition{}
	subdivisions := 0
	var previous [3]*big.Rat
	for i := 0; i < len(cuts)-1; i++ {
		start, end := cuts[i], cuts[i+1]
		local, e := restrictHomogeneousBezier(net, start, end, budget)
		if e != nil {
			return zero, e
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(start, end), big.NewRat(2, 1))
		point, e := exactCurveRationalPoint(c, mid, budget)
		if e != nil {
			return zero, e
		}
		var shifts [2]*big.Int
		for axis, period := range w.period {
			shifts[axis] = new(big.Int)
			if period == nil {
				continue
			}
			q := new(big.Rat).Quo(new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(mins[axis])), period)
			if e := budget.check(q); e != nil {
				return zero, e
			}
			shifts[axis] = floor(q)
			shift := new(big.Rat).Mul(new(big.Rat).SetInt(shifts[axis]), period)
			for j := range local {
				local[j][axis] = new(big.Rat).Sub(local[j][axis], new(big.Rat).Mul(shift, local[j][3]))
				if e := budget.check(local[j][axis]); e != nil {
					return zero, e
				}
			}
		}
		uv, e := exactCurveFromHomogeneousBezier(local, TubeAffineLineOptions{MaxPoles: opts.MaxPoles, MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, budget)
		if e != nil {
			return zero, e
		}
		child := o
		child.MaxOperations = opts.MaxOperations - budget.operations
		child.MaxPieces = o.MaxPieces - len(r.Pieces)
		child.MaxRootSubdivisions = o.MaxRootSubdivisions - subdivisions
		if child.MaxOperations < 1 || child.MaxPieces < 1 || child.MaxRootSubdivisions < 1 {
			return zero, ErrConvergence
		}
		parts, e := w.patch.ComposeParameterCurvePieces(ctx, uv, child)
		if e != nil {
			return zero, e
		}
		budget.operations += parts.ExactOperations
		subdivisions += parts.Subdivisions
		if !parts.Complete || !parts.Continuous {
			return zero, ErrConvergence
		}
		for _, part := range parts.Pieces {
			global := func(t *big.Rat) *big.Rat {
				return new(big.Rat).Add(start, new(big.Rat).Mul(new(big.Rat).Sub(end, start), t))
			}
			a, b := global(part.Start), global(part.End)
			for _, q := range []*big.Rat{a, b} {
				if e := budget.check(q); e != nil {
					return zero, e
				}
			}
			original, e := restrictHomogeneousBezier(net, a, b, budget)
			if e != nil {
				return zero, e
			}
			lifted, e := exactCurveFromHomogeneousBezier(original, TubeAffineLineOptions{MaxPoles: opts.MaxPoles, MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, budget)
			if e != nil {
				return zero, e
			}
			first, e := exactCurveRationalPoint(part.Curve.patch, new(big.Rat), budget)
			if e != nil {
				return zero, e
			}
			last, e := exactCurveRationalPoint(part.Curve.patch, big.NewRat(1, 1), budget)
			if e != nil {
				return zero, e
			}
			if len(r.Pieces) > 0 {
				for axis := 0; axis < 3; axis++ {
					if first[axis].Cmp(previous[axis]) != 0 {
						return zero, ErrConvergence
					}
				}
			}
			previous = last
			r.Pieces = append(r.Pieces, WrappedCurvePiece{Start: a, End: b, UV: lifted, Curve: part.Curve, Cell: part.Cell, PeriodShift: [2]*big.Int{new(big.Int).Set(shifts[0]), new(big.Int).Set(shifts[1])}})
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	r.Complete = true
	r.Continuous = true
	r.ExactOperations = budget.operations
	return r, nil
}
