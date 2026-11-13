package geom

import (
	"context"
	"math/big"
	"sort"
)

// SurfaceCurveCompositionOptions bounds exact quadratic UV composition across
// knot cells. Defaults: 4096 pieces, 8192 root subdivisions, 32 hull levels.
type SurfaceCurveCompositionOptions struct {
	SurfacePatchOptions
	MaxPieces, MaxRootSubdivisions, MaxHullDepth int
}

// SurfaceCurveKnotRoot is an unresolved algebraic normalized parameter crossing.
// Its closed rational interval is never substituted for an exact cut.
type SurfaceCurveKnotRoot struct {
	Axis     int
	Knot     float64
	Interval ExactParameterInterval
}

// SurfaceCurveCompositionPieces retains the standard exact piece collection.
// Complete requires all knot events recovered exactly and all model joins proved.
type SurfaceCurveCompositionPieces struct {
	SurfaceAffineCurvePieces
	Unresolved                     []SurfaceCurveKnotRoot
	Subdivisions, HullSubdivisions int
}

// ComposeParameterCurvePieces composes one degree-one or degree-two exact UV
// Bézier with a continuous multispan rational tensor patch. UV is carried in
// X/Y, Z=0, and parameter domain must be [0,1]. Every knot root is isolated;
// rational roots become exact cuts, coincident knot paths use the right cell
// (last at the upper domain), and unrecovered roots remain ErrConvergence.
// Further rational bisection establishes positive-weight whole-cell control
// hulls. Each returned curve uses local [0,1], with exact global Start/End.
func (p *SurfacePatch) ComposeParameterCurvePieces(ctx context.Context, curve *ExactSplineCurve, o SurfaceCurveCompositionOptions) (r SurfaceCurveCompositionPieces, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || curve == nil || curve.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	if o.MaxPieces == 0 {
		o.MaxPieces = 4096
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxHullDepth == 0 {
		o.MaxHullDepth = 32
	}
	if o.MaxPieces < 1 || o.MaxRootSubdivisions < 1 || o.MaxHullDepth < 1 || o.MaxHullDepth > 64 {
		return r, ErrInvalid
	}
	c := curve.patch
	lo, hi := curve.Domain()
	if lo != 0 || hi != 1 || c.countU != c.degreeU+1 || c.degreeU < 1 || c.degreeU > 2 {
		return r, ErrConvergence
	}
	if len(p.net) > options.MaxPoles || len(c.net) > options.MaxPoles {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	net := make([]exactHomogeneous, c.countU)
	for i := range net {
		net[i] = c.net[i*c.countV]
		if net[i][2].Sign() != 0 {
			return r, ErrInvalid
		}
	}
	d := p.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	knots := [2][]float64{p.knotsU, p.knotsV}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	seen := map[string]bool{"0": true, "1": true}
	for axis := 0; axis < 2; axis++ {
		last := 0.
		haveLast := false
		for _, knot := range knots[axis] {
			if haveLast && knot == last {
				continue
			}
			haveLast = true
			last = knot
			values := make([]*big.Rat, len(net))
			for i, h := range net {
				values[i] = new(big.Rat).Sub(h[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(knot), h[3]))
				if e := budget.check(values[i]); e != nil {
					return r, e
				}
			}
			polynomial, e := bernsteinToPower(values, budget)
			if e != nil {
				return r, e
			}
			if len(polynomial) == 0 {
				continue
			}
			remaining := o.MaxRootSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			steps, e := isolatePolynomialRoots(polynomial, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
				if e := ctx.Err(); e != nil {
					return false, e
				}
				if lo.Cmp(hi) != 0 {
					for _, bound := range []*big.Rat{lo, hi} {
						value, e := polynomialValue(polynomial, bound, budget)
						if e != nil {
							return false, e
						}
						if value.Sign() == 0 {
							return false, nil
						}
					}
				}
				root, e := recoverRationalPolynomialRoot(polynomial, lo, hi, budget)
				if e != nil {
					return false, e
				}
				if root == nil {
					if new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 1000000000000)) > 0 {
						return false, nil
					}
					r.Unresolved = append(r.Unresolved, SurfaceCurveKnotRoot{Axis: axis, Knot: knot, Interval: ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}})
					return true, nil
				}
				key := root.RatString()
				if !seen[key] {
					if len(cuts)-1 >= o.MaxPieces {
						return false, ErrConvergence
					}
					seen[key] = true
					cuts = append(cuts, new(big.Rat).Set(root))
				}
				return true, nil
			})
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
		}
	}
	if len(r.Unresolved) != 0 {
		return r, ErrConvergence
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	var previous [3]*big.Rat
	var compose func(*big.Rat, *big.Rat, int) error
	compose = func(start, end *big.Rat, depth int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(r.Pieces) >= o.MaxPieces {
			return ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(start, end), big.NewRat(2, 1))
		point, e := exactCurveRationalPoint(c, mid, budget)
		if e != nil {
			return e
		}
		var cellBounds [2][2]float64
		for axis := 0; axis < 2; axis++ {
			found := false
			for j := 0; j < len(knots[axis])-1; j++ {
				lo, hi := knots[axis][j], knots[axis][j+1]
				if lo >= hi {
					continue
				}
				if point[axis].Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && (point[axis].Cmp(new(big.Rat).SetFloat64(hi)) < 0 || hi == ranges[axis][1] && point[axis].Cmp(new(big.Rat).SetFloat64(hi)) == 0) {
					cellBounds[axis] = [2]float64{lo, hi}
					found = true
					break
				}
			}
			if !found {
				return ErrDomain
			}
		}
		local, e := restrictHomogeneousBezier(net, start, end, budget)
		if e != nil {
			return e
		}
		contained := true
		for _, h := range local {
			for axis := 0; axis < 2; axis++ {
				if h[axis].Cmp(new(big.Rat).Mul(new(big.Rat).SetFloat64(cellBounds[axis][0]), h[3])) < 0 || h[axis].Cmp(new(big.Rat).Mul(new(big.Rat).SetFloat64(cellBounds[axis][1]), h[3])) > 0 {
					contained = false
				}
			}
		}
		if !contained {
			if depth >= o.MaxHullDepth {
				return ErrConvergence
			}
			r.HullSubdivisions++
			if e := compose(start, mid, depth+1); e != nil {
				return e
			}
			return compose(mid, end, depth+1)
		}
		cell := ParameterRectangle{UMin: cellBounds[0][0], UMax: cellBounds[0][1], VMin: cellBounds[1][0], VMax: cellBounds[1][1]}
		patch, e := trimPatchBudget(p, cell, budget)
		if e != nil {
			return e
		}
		uv, e := exactCurveFromHomogeneousBezier(local, TubeAffineLineOptions{MaxPoles: options.MaxPoles, MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits}, budget)
		if e != nil {
			return e
		}
		child := options
		child.MaxOperations -= budget.operations
		if child.MaxOperations < 1 {
			return ErrConvergence
		}
		model, e := patch.ComposeParameterCurve(ctx, uv, child)
		if e != nil {
			return e
		}
		budget.operations += model.ExactOperations()
		first, e := exactCurveRationalPoint(model.patch, new(big.Rat), budget)
		if e != nil {
			return e
		}
		last, e := exactCurveRationalPoint(model.patch, big.NewRat(1, 1), budget)
		if e != nil {
			return e
		}
		if len(r.Pieces) > 0 {
			for axis := 0; axis < 3; axis++ {
				if first[axis].Cmp(previous[axis]) != 0 {
					return ErrConvergence
				}
			}
		}
		previous = last
		r.Pieces = append(r.Pieces, SurfaceAffineCurvePiece{Start: new(big.Rat).Set(start), End: new(big.Rat).Set(end), Curve: model, Cell: cell})
		return nil
	}
	for i := 0; i < len(cuts)-1; i++ {
		if e := compose(cuts[i], cuts[i+1], 0); e != nil {
			return r, e
		}
	}
	r.Continuous = true
	r.Complete = true
	return r, nil
}
