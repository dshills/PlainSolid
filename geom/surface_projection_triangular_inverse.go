package geom

import (
	"context"
	"math/big"
	"runtime"
)

// TriangularSurfaceInverseOptions requires ProjectionAxes[0] to be affine in
// FixedParameterAxis (0=U,1=V) and independent of the other parameter. Limits use
// RationalSurfaceInverseOptions defaults; ParameterTolerance defaults to1e-10
// in original varying-parameter units, MaxRootSubdivisions to8192. Embedded
// Candidates are unused: this routine derives roots directly.
type TriangularSurfaceInverseOptions struct {
	RationalSurfaceInverseOptions
	FixedParameterAxis  int
	ParameterTolerance  float64
	MaxRootSubdivisions int
}

// TriangularSurfaceInverse proves a unique preimage when Found, including
// algebraic varying parameters. Lower/Upper are exact original UV enclosures;
// Parameters is populated per coordinate only for rationally recovered values.
// ModelBounds encloses the actual inverse point, not an approximate residual.
type TriangularSurfaceInverse struct {
	// FullModelQuery distinguishes full 3D membership from projection inversion.
	// ModelPointValidated is set only on a Found exact full-point certificate.
	FullModelQuery, ModelPointValidated bool
	TargetPoint                         [3]*big.Rat
	Found, Excluded, Complete           bool
	Parameters, Lower, Upper            [2]*big.Rat
	ModelBounds                         Box
	Injectivity                         SurfaceInjectivity
	ExactOperations, RootSubdivisions   int
}

// InvertTriangularSurfaceProjectionRational verifies the affine coordinate
// identity on every exact Bézier cell, solves that coordinate without rounding,
// and isolates the remaining univariate homogeneous residual. A separate global
// injectivity certificate ensures that an accepted contact is the unique preimage.
func InvertTriangularSurfaceProjectionRational(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, o TriangularSurfaceInverseOptions) (TriangularSurfaceInverse, error) {
	return invertTriangularSurfaceRational(ctx, p, query, nil, o)
}

// InvertTriangularSurfacePointRational proves full model-point membership by
// exact GCD of both remaining coordinate residuals after the affine-axis solve.
// Found is exact even when the unique UV is algebraic. Excluded concerns the
// full 3D point; its projected coordinates may still have a surface preimage.
func InvertTriangularSurfacePointRational(ctx context.Context, p *SurfacePatch, point [3]*big.Rat, o TriangularSurfaceInverseOptions) (TriangularSurfaceInverse, error) {
	for _, x := range point {
		if x == nil {
			return TriangularSurfaceInverse{}, ErrInvalid
		}
	}
	axes := o.ProjectionAxes
	if axes == ([2]int{}) {
		axes = [2]int{0, 1}
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return TriangularSurfaceInverse{}, ErrInvalid
	}
	return invertTriangularSurfaceRational(ctx, p, [2]*big.Rat{point[axes[0]], point[axes[1]]}, &point, o)
}
func invertTriangularSurfaceRational(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, model *[3]*big.Rat, o TriangularSurfaceInverseOptions) (r TriangularSurfaceInverse, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || query[0] == nil || query[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ProjectionAxes == ([2]int{}) {
		o.ProjectionAxes = [2]int{0, 1}
	}
	if o.FixedParameterAxis < 0 || o.FixedParameterAxis > 1 || o.ProjectionAxes[0] < 0 || o.ProjectionAxes[0] > 2 || o.ProjectionAxes[1] < 0 || o.ProjectionAxes[1] > 2 || o.ProjectionAxes[0] == o.ProjectionAxes[1] {
		return r, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxDepth < 0 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxRootSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for _, x := range query {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	if model != nil {
		r.FullModelQuery = true
		for axis, x := range *model {
			if e := b.check(x); e != nil {
				return r, e
			}
			r.TargetPoint[axis] = new(big.Rat).Set(x)
		}
	}
	d := p.Domain()
	fixed := o.FixedParameterAxis
	varying := 1 - fixed
	// Preserve original domain endpoints independently of subsequent cell work.
	fixedLo, fixedHi := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	if fixed == 1 {
		fixedLo.SetFloat64(d.VMin)
		fixedHi.SetFloat64(d.VMax)
	}
	var ends [2]*big.Rat
	for side := 0; side < 2; side++ {
		uv := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
		uv[fixed].Set(fixedLo)
		if side == 1 {
			uv[fixed].Set(fixedHi)
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return r, e
		}
		ends[side] = jet.Point[o.ProjectionAxes[0]]
	}
	scale := new(big.Rat).Quo(new(big.Rat).Sub(ends[1], ends[0]), new(big.Rat).Sub(fixedHi, fixedLo))
	if scale.Sign() == 0 {
		return r, ErrConvergence
	}
	offset := new(big.Rat).Sub(ends[0], new(big.Rat).Mul(scale, fixedLo))
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		cd := cell.Domain()
		ranges := [2][2]float64{{cd.UMin, cd.UMax}, {cd.VMin, cd.VMax}}
		n, m := cell.countU, cell.countV
		if fixed == 1 {
			n, m = m, n
		}
		linear := rationalPolynomial{new(big.Rat).Add(offset, new(big.Rat).Mul(scale, new(big.Rat).SetFloat64(ranges[fixed][0]))), new(big.Rat).Mul(scale, new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[fixed][1]), new(big.Rat).SetFloat64(ranges[fixed][0])))}
		for j := 0; j < m; j++ {
			x, w := make([]*big.Rat, n), make([]*big.Rat, n)
			for i := 0; i < n; i++ {
				index := i*cell.countV + j
				if fixed == 1 {
					index = j*cell.countV + i
				}
				x[i] = cell.net[index][o.ProjectionAxes[0]]
				w[i] = cell.net[index][3]
			}
			xp, e := bernsteinToPower(x, b)
			if e != nil {
				return r, e
			}
			wp, e := bernsteinToPower(w, b)
			if e != nil {
				return r, e
			}
			expected, e := polynomialProduct(linear, wp, b)
			if e != nil {
				return r, e
			}
			difference, e := sharedProjectionSubtract(xp, expected, b)
			if e != nil {
				return r, e
			}
			if len(difference) != 0 {
				return r, ErrConvergence
			}
		}
	}
	fixedValue := new(big.Rat).Quo(new(big.Rat).Sub(query[0], offset), scale)
	if e := b.check(fixedValue); e != nil {
		return r, e
	}
	if fixedValue.Cmp(fixedLo) < 0 || fixedValue.Cmp(fixedHi) > 0 {
		r.Excluded = true
		r.Complete = true
		return r, nil
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Injectivity, e = CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{ProjectionAxes: o.ProjectionAxes, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Injectivity.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Injectivity.ProvenInjective {
		return r, ErrConvergence
	}
	seen := map[ParameterSpan]bool{}
	rootSeen := map[string]bool{}
	hits := 0
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		cd := cell.Domain()
		ranges := [2][2]float64{{cd.UMin, cd.UMax}, {cd.VMin, cd.VMax}}
		lo, hi := new(big.Rat).SetFloat64(ranges[fixed][0]), new(big.Rat).SetFloat64(ranges[fixed][1])
		if fixedValue.Cmp(lo) < 0 || fixedValue.Cmp(hi) > 0 {
			continue
		}
		span := ParameterSpan{Start: ranges[varying][0], End: ranges[varying][1]}
		if seen[span] {
			continue
		}
		seen[span] = true
		t := new(big.Rat).Quo(new(big.Rat).Sub(fixedValue, lo), new(big.Rat).Sub(hi, lo))
		n, m := cell.countU, cell.countV
		if fixed == 1 {
			n, m = m, n
		}
		net := make([]exactHomogeneous, m)
		for j := 0; j < m; j++ {
			for axis := 0; axis < 4; axis++ {
				values := make([]*big.Rat, n)
				for i := range values {
					index := i*cell.countV + j
					if fixed == 1 {
						index = j*cell.countV + i
					}
					values[i] = cell.net[index][axis]
				}
				net[j][axis], e = scalarBernsteinValue(values, t, b)
				if e != nil {
					return r, e
				}
			}
		}
		residual := make([]*big.Rat, m)
		for i, h := range net {
			residual[i] = new(big.Rat).Sub(h[o.ProjectionAxes[1]], new(big.Rat).Mul(query[1], h[3]))
			if e := b.check(residual[i]); e != nil {
				return r, e
			}
		}
		poly, e := bernsteinToPower(residual, b)
		if e != nil {
			return r, e
		}
		if model != nil {
			third := 3 - o.ProjectionAxes[0] - o.ProjectionAxes[1]
			coefficients := make([]*big.Rat, m)
			for i, h := range net {
				coefficients[i] = new(big.Rat).Sub(h[third], new(big.Rat).Mul((*model)[third], h[3]))
				runtime.KeepAlive(&h) // golang/go#82026: keeps h in the stack map. Remove once the Go fix is in use.
				if e := b.check(coefficients[i]); e != nil {
					return r, e
				}
			}
			other, e := bernsteinToPower(coefficients, b)
			if e != nil {
				return r, e
			}
			poly, e = polynomialGCD(poly, other, b)
			if e != nil {
				return r, e
			}
		}
		if len(poly) == 0 {
			return r, ErrConvergence
		}
		if len(poly) == 1 {
			continue
		}
		rootBudget := o.MaxRootSubdivisions - r.RootSubdivisions
		if rootBudget < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(poly, new(big.Rat), big.NewRat(1, 1), b, rootBudget, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(poly, lo, hi, b)
			if e != nil {
				return false, e
			}
			lower, upper := parameterMap(span, lo), parameterMap(span, hi)
			if candidate == nil && new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			if candidate != nil {
				parameter := parameterMap(span, candidate)
				key := parameter.RatString()
				if rootSeen[key] {
					return true, nil
				}
				rootSeen[key] = true
				r.Parameters[varying] = parameter
				lower, upper = new(big.Rat).Set(parameter), new(big.Rat).Set(parameter)
				lo, hi = candidate, candidate
			}
			hits++
			if hits > 1 {
				return false, ErrConvergence
			}
			r.Parameters[fixed] = new(big.Rat).Set(fixedValue)
			r.Lower[fixed] = new(big.Rat).Set(fixedValue)
			r.Upper[fixed] = new(big.Rat).Set(fixedValue)
			r.Lower[varying], r.Upper[varying] = lower, upper
			restricted, e := restrictHomogeneousBezier(net, lo, hi, b)
			if e != nil {
				return false, e
			}
			var minima, maxima [3]*big.Rat
			for _, h := range restricted {
				for axis := 0; axis < 3; axis++ {
					x := new(big.Rat).Quo(h[axis], h[3])
					if e := b.check(x); e != nil {
						return false, e
					}
					if minima[axis] == nil || x.Cmp(minima[axis]) < 0 {
						minima[axis] = x
					}
					if maxima[axis] == nil || x.Cmp(maxima[axis]) > 0 {
						maxima[axis] = x
					}
				}
			}
			r.ModelBounds = Box{Min: Vec3{rationalFloatBound(minima[0], true), rationalFloatBound(minima[1], true), rationalFloatBound(minima[2], true)}, Max: Vec3{rationalFloatBound(maxima[0], false), rationalFloatBound(maxima[1], false), rationalFloatBound(maxima[2], false)}}
			return true, nil
		})
		r.RootSubdivisions += steps
		if e != nil {
			return r, e
		}
	}
	r.Complete = true
	r.Found = hits == 1
	r.Excluded = hits == 0
	if r.Found && model != nil {
		r.ModelPointValidated = true
		r.ModelBounds = Box{Min: Vec3{rationalFloatBound((*model)[0], true), rationalFloatBound((*model)[1], true), rationalFloatBound((*model)[2], true)}, Max: Vec3{rationalFloatBound((*model)[0], false), rationalFloatBound((*model)[1], false), rationalFloatBound((*model)[2], false)}}
	}
	return r, nil
}
