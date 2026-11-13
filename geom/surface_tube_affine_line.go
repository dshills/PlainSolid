package geom

import (
	"context"
	"math"
	"math/big"
)

// TubeAffineLineOptions limits exact diagonal restrictions and identities.
// Zero limits select 4096 poles, 4M operations and 16384 coefficient bits.
type TubeAffineLineOptions struct{ MaxPoles, MaxOperations, MaxCoefficientBits int }

// TubeAffineLineCertificate proves that the complete verified tube follows the
// supplied affine parameter line and its model image is exactly the affine
// segment ModelStart+(ModelEnd-ModelStart)*t, t in [0,1]. Curve is an immutable
// exact degree-one descriptor on [0,1]. Endpoint rationals are caller-owned and
// do not alias Curve or the tube. This is stronger than geometric collinearity;
// straight images with nonlinear speed can remain unproved by this operation.
type TubeAffineLineCertificate struct {
	Proven               bool
	Start, End           [4]*big.Rat
	ModelStart, ModelEnd [3]*big.Rat
	Curve                *ExactSplineCurve
	ExactOperations      int
}

// CertifyAffineParameterLine checks exact supplied four-parameter endpoints.
// Both lie in the tube and the free coordinate spans its entire interval, in
// either direction. Exact tensor restriction and homogeneous polynomial identity
// prove equality between charts and with the proposed model endpoint segment.
// The immutable tube's uniqueness then identifies this curve with the complete
// branch. No sampled inference or rounded endpoint reconstruction is used.
func (t VerifiedSurfaceTube) CertifyAffineParameterLine(ctx context.Context, start, end [4]*big.Rat, o TubeAffineLineOptions) (r TubeAffineLineCertificate, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	ranges := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})
	for axis := 0; axis < 4; axis++ {
		if start[axis] == nil || end[axis] == nil {
			return r, ErrInvalid
		}
		lo, hi := new(big.Rat).SetFloat64(ranges[axis][0]), new(big.Rat).SetFloat64(ranges[axis][1])
		for _, x := range []*big.Rat{start[axis], end[axis]} {
			if e := budget.check(x); e != nil {
				return r, e
			}
			if x.Cmp(lo) < 0 || x.Cmp(hi) > 0 {
				return r, ErrDomain
			}
		}
		if axis == t.axis && !((start[axis].Cmp(lo) == 0 && end[axis].Cmp(hi) == 0) || (start[axis].Cmp(hi) == 0 && end[axis].Cmp(lo) == 0)) {
			return r, ErrDomain
		}
	}
	makeCurve := func(p *SurfacePatch, offset int) (*ExactSplineCurve, error) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return nil, ErrDomain
		}
		degree := p.degreeU + p.degreeV
		if degree+1 > o.MaxPoles/2 {
			return nil, ErrConvergence
		}
		var lower, upper [2]*big.Rat
		var reversed [2]bool
		for axis := 0; axis < 2; axis++ {
			index := offset + axis
			origin := new(big.Rat).SetFloat64(ranges[index][0])
			width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[index][1]), origin)
			lo := new(big.Rat).Quo(new(big.Rat).Sub(start[index], origin), width)
			hi := new(big.Rat).Quo(new(big.Rat).Sub(end[index], origin), width)
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
				reversed[axis] = true
			}
			lower[axis], upper[axis] = lo, hi
		}
		net, e := restrictComparisonTensor(comparisonTensor{nu: p.countU, nv: p.countV, net: p.net}, lower[0], upper[0], lower[1], upper[1], budget)
		if e != nil {
			return nil, e
		}
		diagonal := make([]exactHomogeneous, degree+1)
		for i := range diagonal {
			for axis := 0; axis < 4; axis++ {
				diagonal[i][axis] = new(big.Rat)
			}
		}
		for i := 0; i < p.countU; i++ {
			for j := 0; j < p.countV; j++ {
				u, v := i, j
				if reversed[0] {
					u = p.countU - 1 - i
				}
				if reversed[1] {
					v = p.countV - 1 - j
				}
				h := net[u*p.countV+v]
				numerator := new(big.Int).Mul(new(big.Int).Binomial(int64(p.degreeU), int64(i)), new(big.Int).Binomial(int64(p.degreeV), int64(j)))
				factor := new(big.Rat).SetFrac(numerator, new(big.Int).Binomial(int64(degree), int64(i+j)))
				if e := budget.check(factor); e != nil {
					return nil, e
				}
				for axis := 0; axis < 4; axis++ {
					diagonal[i+j][axis].Add(diagonal[i+j][axis], new(big.Rat).Mul(factor, h[axis]))
					if e := budget.check(diagonal[i+j][axis]); e != nil {
						return nil, e
					}
				}
			}
		}
		return exactCurveFromHomogeneousBezier(diagonal, o, budget)
	}
	a, e := makeCurve(t.a, 0)
	if e != nil {
		return r, e
	}
	b, e := makeCurve(t.b, 2)
	if e != nil {
		return r, e
	}
	equal := func(a, b *ExactSplineCurve) (bool, error) {
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return false, ErrConvergence
		}
		comparison, e := CompareExactCurvesContext(ctx, a, b, ExactCurveComparisonOptions{MaxSpans: 1, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += comparison.ExactOperations
		return comparison.Complete && comparison.Equal, e
	}
	same, e := equal(a, b)
	if e != nil {
		return r, e
	}
	if !same {
		return r, ErrConvergence
	}
	endpoints := []exactHomogeneous{a.patch.net[0], a.patch.net[(a.patch.countU-1)*a.patch.countV]}
	line := make([]exactHomogeneous, 2)
	different := false
	for i, h := range endpoints {
		line[i][3] = big.NewRat(1, 1)
		for axis := 0; axis < 3; axis++ {
			line[i][axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(line[i][axis]); e != nil {
				return r, e
			}
			if i == 1 && line[0][axis].Cmp(line[1][axis]) != 0 {
				different = true
			}
		}
	}
	if !different {
		return r, ErrDegenerate
	}
	model, e := exactCurveFromHomogeneousBezier(line, o, budget)
	if e != nil {
		return r, e
	}
	same, e = equal(a, model)
	if e != nil {
		return r, e
	}
	if !same {
		return r, ErrConvergence
	}
	for axis := 0; axis < 4; axis++ {
		r.Start[axis] = new(big.Rat).Set(start[axis])
		r.End[axis] = new(big.Rat).Set(end[axis])
	}
	for axis := 0; axis < 3; axis++ {
		r.ModelStart[axis] = new(big.Rat).Set(line[0][axis])
		r.ModelEnd[axis] = new(big.Rat).Set(line[1][axis])
	}
	r.Curve = model
	r.Proven = true
	return r, nil
}

func exactCurveFromHomogeneousBezier(net []exactHomogeneous, o TubeAffineLineOptions, b *polynomialBudget) (*ExactSplineCurve, error) {
	if len(net) < 2 || len(net) > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	doubled := make([]exactHomogeneous, 2*len(net))
	for i, h := range net {
		if h[3].Sign() <= 0 {
			return nil, ErrInvalid
		}
		for axis := 0; axis < 4; axis++ {
			doubled[2*i][axis] = new(big.Rat).Set(h[axis])
			doubled[2*i+1][axis] = new(big.Rat).Set(h[axis])
		}
	}
	knots := make([]float64, 2*len(net))
	for i := len(net); i < len(knots); i++ {
		knots[i] = 1
	}
	p := &SurfacePatch{degreeU: len(net) - 1, degreeV: 1, countU: len(net), countV: 2, knotsU: knots, knotsV: []float64{0, 0, 1, 1}, net: doubled, options: SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	return &SurfaceIsoCurve{patch: p, fixedAxis: -1, fixedParameter: math.NaN(), operations: b.operations}, nil
}
