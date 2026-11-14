package geom

import (
	"context"
	"math/big"
)

// ExactTensorSurfaceRecipe stores original clamped knots and canonical rational
// homogeneous controls. It contains no asserted bounds, regularity or topology.
type ExactTensorSurfaceRecipe struct {
	Version                          int
	DegreeU, DegreeV, CountU, CountV int
	KnotsU, KnotsV                   []float64
	Controls                         [][4]string
}

func (p *SurfacePatch) ExportExactTensorRecipe() (ExactTensorSurfaceRecipe, error) {
	if p == nil {
		return ExactTensorSurfaceRecipe{}, ErrInvalid
	}
	r := ExactTensorSurfaceRecipe{Version: 1, DegreeU: p.degreeU, DegreeV: p.degreeV, CountU: p.countU, CountV: p.countV, KnotsU: append([]float64(nil), p.knotsU...), KnotsV: append([]float64(nil), p.knotsV...), Controls: make([][4]string, len(p.net))}
	for i, h := range p.net {
		for j, v := range h {
			r.Controls[i][j] = v.RatString()
		}
	}
	return r, nil
}

// ImportExactTensorSurfaceRecipe independently validates finite clamped C0
// knot structure, dimensions, positive weights and all rational input bounds.
// The control net never passes through floating Euclidean poles.
func ImportExactTensorSurfaceRecipe(ctx context.Context, r ExactTensorSurfaceRecipe, o SurfacePatchOptions) (*SurfacePatch, error) {
	if ctx == nil || r.Version != 1 || r.DegreeU < 1 || r.DegreeU > 64 || r.DegreeV < 1 || r.DegreeV > 64 || r.CountU < r.DegreeU+1 || r.CountV < r.DegreeV+1 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if r.CountU > opts.MaxPoles/r.CountV {
		return nil, ErrConvergence
	}
	count := r.CountU * r.CountV
	if len(r.Controls) != count || validateKnots(r.DegreeU, r.CountU, r.KnotsU) != nil || validateKnots(r.DegreeV, r.CountV, r.KnotsV) != nil || !projectionKnotsSupported(r.DegreeU, r.CountU, r.KnotsU) || !projectionKnotsSupported(r.DegreeV, r.CountV, r.KnotsV) {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	p := &SurfacePatch{degreeU: r.DegreeU, degreeV: r.DegreeV, countU: r.CountU, countV: r.CountV, knotsU: append([]float64(nil), r.KnotsU...), knotsV: append([]float64(nil), r.KnotsV...), net: make([]exactHomogeneous, count), options: opts}
	for i, h := range r.Controls {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		for j, text := range h {
			if len(text) > opts.MaxCoefficientBits+3 {
				return nil, ErrConvergence
			}
			for _, ch := range text {
				if ch != '-' && ch != '/' && (ch < '0' || ch > '9') {
					return nil, ErrInvalid
				}
			}
			v, ok := new(big.Rat).SetString(text)
			if !ok || v.RatString() != text {
				return nil, ErrInvalid
			}
			if e = b.check(v); e != nil {
				return nil, e
			}
			p.net[i][j] = v
		}
		if p.net[i][3].Sign() <= 0 {
			return nil, ErrInvalid
		}
	}
	if e = p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	return p, nil
}

// ExportExactTensorRecipeBounded checks coefficients before textual allocation.
// The conservative byte estimate includes decimal numerators/denominators and
// knot fields; callers must still bound their complete enclosing JSON document.
func (p *SurfacePatch) ExportExactTensorRecipeBounded(ctx context.Context, maxBytes int64, o SurfacePatchOptions) (ExactTensorSurfaceRecipe, int, error) {
	var zero ExactTensorSurfaceRecipe
	if ctx == nil || p == nil || maxBytes < 1 {
		return zero, 0, ErrInvalid
	}
	opts, e := patchOptions(o)
	if e != nil {
		return zero, 0, e
	}
	if len(p.net) > opts.MaxPoles {
		return zero, 0, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	size := int64(256) + int64(len(p.knotsU)+len(p.knotsV))*32
	if size > maxBytes {
		return zero, 0, ErrConvergence
	}
	for _, h := range p.net {
		if e = ctx.Err(); e != nil {
			return zero, 0, e
		}
		for _, v := range h {
			if e = b.check(v); e != nil {
				return zero, 0, e
			}
			// log10(2)<1/3 gives an outward bound for canonical decimal digits.
			size += int64(v.Num().BitLen()/3 + v.Denom().BitLen()/3 + 12)
			if size > maxBytes {
				return zero, 0, ErrConvergence
			}
		}
	}
	r, e := p.ExportExactTensorRecipe()
	return r, b.operations, e
}
