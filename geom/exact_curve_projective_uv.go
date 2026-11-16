package geom

import (
	"context"
	"math/big"
)

// ProjectiveUVAxisMap maps one original coordinate s to
// (Numerator[0]*s+Numerator[1])/(Denominator[0]*s+Denominator[1]).
// SourceAxis selects original U=0 or V=1. Coefficients are read-only.
type ProjectiveUVAxisMap struct {
	SourceAxis             int
	Numerator, Denominator [2]*big.Rat
}

// ProjectiveUVMap maps source U,V to two target coordinates. Each axis
// must use a different source coordinate and have a nonzero determinant.
type ProjectiveUVMap [2]ProjectiveUVAxisMap

// TransformProjectiveUV maps the exact original XY parameter curve by
// independent rational projective coordinate maps. It retains physical
// curve parameters and all original break stations. The denominators must
// have strictly positive Bernstein controls on every original Bézier span.
// This is a sufficient pole exclusion proof, not a general root solver.
// Nonaffine output can double the degree and retains separate Bézier pieces.
func (c *SurfaceIsoCurve) TransformProjectiveUV(ctx context.Context, m ProjectiveUVMap,
	o SurfacePatchOptions) (*ExactSplineCurve, error) {
	if ctx == nil || c == nil || c.patch == nil || m[0].SourceAxis < 0 || m[0].SourceAxis > 1 ||
		m[1].SourceAxis != 1-m[0].SourceAxis {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limits, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := boundedRatArithmetic{b: b}
	affine := true
	for _, q := range m {
		for _, coef := range [4]*big.Rat{q.Numerator[0], q.Numerator[1], q.Denominator[0], q.Denominator[1]} {
			if coef == nil {
				return nil, ErrInvalid
			}
			if err := b.check(coef); err != nil {
				return nil, err
			}
		}
		det := a.sub(a.mul(q.Numerator[0], q.Denominator[1]), a.mul(q.Numerator[1], q.Denominator[0]))
		if a.err != nil {
			return nil, a.err
		}
		if det.Sign() == 0 {
			return nil, ErrDegenerate
		}
		affine = affine && q.Denominator[0].Sign() == 0
	}
	source := *c.patch
	source.options.MaxPoles = min(source.options.MaxPoles, limits.MaxPoles)
	if len(source.net) > source.options.MaxPoles {
		return nil, ErrConvergence
	}
	for _, h := range source.net {
		if h[2].Sign() != 0 {
			return nil, ErrDomain
		}
	}
	if affine {
		var mat [3][4]*big.Rat
		for i := range mat {
			for j := range mat[i] {
				mat[i][j] = new(big.Rat)
			}
		}
		for i, q := range m {
			if q.Denominator[1].Sign() <= 0 {
				return nil, ErrDomain
			}
			mat[i][q.SourceAxis] = a.quo(q.Numerator[0], q.Denominator[1])
			mat[i][3] = a.quo(q.Numerator[1], q.Denominator[1])
		}
		if a.err != nil {
			return nil, a.err
		}
		limits.MaxOperations -= b.operations
		if limits.MaxOperations < 1 {
			return nil, ErrConvergence
		}
		result, err := c.TransformRational(ctx, mat, limits)
		if err != nil {
			return nil, err
		}
		result.operations += b.operations
		return result, nil
	}
	degree := 2 * source.degreeU
	if degree > 64 {
		return nil, ErrConvergence
	}
	p := &SurfacePatch{degreeU: degree, degreeV: 1, countV: 2, knotsV: []float64{0, 0, 1, 1}, options: limits}
	domain := source.Domain()
	for range degree + 1 {
		p.knotsU = append(p.knotsU, domain.UMin)
	}
	stack := []*SurfacePatch{&source}
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := len(stack) - 1
		span := stack[i]
		stack = stack[:i]
		if k, ok := internalPatchKnot(span.knotsU, span.degreeU, span.countU); ok {
			minimum := (len(stack)+2)*degree*2 + len(p.net)
			if len(p.net) == 0 {
				minimum += 2
			}
			if minimum > limits.MaxPoles {
				return nil, ErrConvergence
			}
			l, h, err := span.split(k, true, b)
			if err != nil {
				return nil, err
			}
			stack = append(stack, h, l)
			continue
		}
		if span.countU != span.degreeU+1 {
			return nil, ErrDomain
		}
		extra := 2 * degree
		if len(p.net) == 0 {
			extra += 2
		}
		if len(p.net)+extra > limits.MaxPoles {
			return nil, ErrConvergence
		}
		controls, err := projectiveUVCurveSpan(span, m, b)
		if err != nil {
			return nil, err
		}
		if len(p.net) != 0 {
			for axis, q := range p.net[len(p.net)-1] {
				if controls[0][axis].Cmp(q) != 0 {
					return nil, ErrDomain
				}
			}
			controls = controls[1:]
			for range degree {
				p.knotsU = append(p.knotsU, span.Domain().UMin)
			}
		}
		for _, h := range controls {
			p.net = append(p.net, h, h)
		}
	}
	for range degree + 1 {
		p.knotsU = append(p.knotsU, domain.UMax)
	}
	p.countU = len(p.net) / 2
	if err := p.computeBounds(b); err != nil {
		return nil, err
	}
	p.operations = b.operations
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SurfaceIsoCurve{p, c.fixedAxis, c.fixedParameter, b.operations}, nil
}

func projectiveUVCurveSpan(p *SurfacePatch, m ProjectiveUVMap, b *polynomialBudget) ([]exactHomogeneous, error) {
	a := boundedRatArithmetic{b: b}
	var num, den [2][]*big.Rat
	for axis, q := range m {
		num[axis], den[axis] = make([]*big.Rat, p.countU), make([]*big.Rat, p.countU)
		for i := 0; i < p.countU; i++ {
			h := p.net[i*p.countV]
			x, w := h[q.SourceAxis], h[3]
			num[axis][i] = a.add(a.mul(q.Numerator[0], x), a.mul(q.Numerator[1], w))
			den[axis][i] = a.add(a.mul(q.Denominator[0], x), a.mul(q.Denominator[1], w))
			if a.err != nil {
				return nil, a.err
			}
			if den[axis][i].Sign() <= 0 {
				return nil, ErrDomain
			}
		}
	}
	x, err := exactBernsteinScalarProduct(num[0], den[1], b)
	if err != nil {
		return nil, err
	}
	y, err := exactBernsteinScalarProduct(num[1], den[0], b)
	if err != nil {
		return nil, err
	}
	w, err := exactBernsteinScalarProduct(den[0], den[1], b)
	if err != nil {
		return nil, err
	}
	zero := new(big.Rat)
	controls := make([]exactHomogeneous, len(w))
	for i := range controls {
		controls[i] = exactHomogeneous{x[i], y[i], zero, w[i]}
	}
	return controls, nil
}
