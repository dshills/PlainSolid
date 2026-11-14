package geom

import (
	"context"
	"math/big"
)

// NewExactHomogeneousBezierSurface retains positive (w*x,w*y,w*z,w) controls
// without float pole conversion or weight normalization. U-major controls have
// (degreeU+1)*(degreeV+1) entries; each degree is in [1,64]. The domain is [0,1]².
// SurfacePatch Eval/Partials provide numerical representatives; EvaluateRational
// and the stored homogeneous geometry remain exact. No regularity is implied.
func NewExactHomogeneousBezierSurface(ctx context.Context, degreeU, degreeV int, controls [][4]*big.Rat, o SurfacePatchOptions) (*SurfacePatch, error) {
	if ctx == nil || degreeU < 1 || degreeU > 64 || degreeV < 1 || degreeV > 64 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	count := (degreeU + 1) * (degreeV + 1)
	if len(controls) != count {
		return nil, ErrInvalid
	}
	if count > opts.MaxPoles {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	net := make([]exactHomogeneous, count)
	for i, p := range controls {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j, v := range p {
			if v == nil {
				return nil, ErrInvalid
			}
			if e := b.check(v); e != nil {
				return nil, e
			}
			net[i][j] = new(big.Rat).Set(v)
		}
		if net[i][3].Sign() <= 0 {
			return nil, ErrInvalid
		}
	}
	knots := func(degree int) []float64 {
		k := make([]float64, 2*(degree+1))
		for i := degree + 1; i < len(k); i++ {
			k[i] = 1
		}
		return k
	}
	p := &SurfacePatch{degreeU: degreeU, degreeV: degreeV, countU: degreeU + 1, countV: degreeV + 1, knotsU: knots(degreeU), knotsV: knots(degreeV), net: net, options: opts}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	return p, nil
}

// ExactBezierSurfaceRecipe is a bounded lossless geometric chart recipe, not a
// shell/topology serialization. Rational strings use canonical RatString form.
// Import independently validates dimensions, positive weights, finite domain,
// and resource limits. Display bounds or derivatives are never trusted input.
type ExactBezierSurfaceRecipe struct {
	Version          int
	DegreeU, DegreeV int
	Domain           ParameterRectangle
	Controls         [][4]string
}

func (p *SurfacePatch) ExportExactBezierRecipe() (ExactBezierSurfaceRecipe, error) {
	var zero ExactBezierSurfaceRecipe
	if p == nil || p.degreeU < 1 || p.degreeV < 1 || p.degreeU > 64 || p.degreeV > 64 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) != p.countU*p.countV {
		return zero, ErrDomain
	}
	r := ExactBezierSurfaceRecipe{Version: 1, DegreeU: p.degreeU, DegreeV: p.degreeV, Domain: p.Domain(), Controls: make([][4]string, len(p.net))}
	for i, h := range p.net {
		for j, v := range h {
			r.Controls[i][j] = v.RatString()
		}
	}
	return r, nil
}

// ImportExactBezierSurfaceRecipe bounds textual coefficients before parsing.
// Original parameter units are preserved; no control value is rounded.
func ImportExactBezierSurfaceRecipe(ctx context.Context, r ExactBezierSurfaceRecipe, o SurfacePatchOptions) (*SurfacePatch, error) {
	if ctx == nil || r.Version != 1 || r.DegreeU < 1 || r.DegreeU > 64 || r.DegreeV < 1 || r.DegreeV > 64 || !r.Domain.Valid() {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	count := (r.DegreeU + 1) * (r.DegreeV + 1)
	if len(r.Controls) != count {
		return nil, ErrInvalid
	}
	if count > opts.MaxPoles {
		return nil, ErrConvergence
	}
	// One sign plus decimal numerator/denominator and slash. This inexpensive
	// cap precedes big integer parsing; exact bit limits are checked afterwards.
	h := make([][4]*big.Rat, count)
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for i, p := range r.Controls {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j, text := range p {
			if len(text) == 0 {
				return nil, ErrInvalid
			}
			if len(text) > opts.MaxCoefficientBits+3 {
				return nil, ErrConvergence
			}
			// Canonical rational syntax excludes exponent expansion and decimal parsing.
			for _, ch := range text {
				if ch != '-' && ch != '/' && (ch < '0' || ch > '9') {
					return nil, ErrInvalid
				}
			}
			v, ok := new(big.Rat).SetString(text)
			if !ok || v.RatString() != text {
				return nil, ErrInvalid
			}
			if e := b.check(v); e != nil {
				return nil, e
			}
			h[i][j] = v
		}
	}
	remaining := opts
	remaining.MaxOperations -= b.operations
	if remaining.MaxOperations < 1 {
		return nil, ErrConvergence
	}
	patch, e := NewExactHomogeneousBezierSurface(ctx, r.DegreeU, r.DegreeV, h, remaining)
	if e != nil {
		return nil, e
	}
	// A single Bezier cell's knot replacement changes parameter units only.
	for i := range patch.knotsU {
		patch.knotsU[i] = r.Domain.UMin
		if i > patch.degreeU {
			patch.knotsU[i] = r.Domain.UMax
		}
	}
	for i := range patch.knotsV {
		patch.knotsV[i] = r.Domain.VMin
		if i > patch.degreeV {
			patch.knotsV[i] = r.Domain.VMax
		}
	}
	patch.options = opts
	patch.operations += b.operations
	return patch, nil
}
