package geom

import (
	"context"
	"math/big"
)

// CertifySurfaceSupportingQuadric proves the sign and complete rectangular
// boundary zero pattern of an exact quadric composed with a rational patch.
// The present proof accepts a single Bézier rectangle. Positive homogeneous
// weights allow sign tests on the exact tensor Bernstein numerator.
func CertifySurfaceSupportingQuadric(ctx context.Context, p *SurfacePatch, q ExactQuadric, o SurfaceSupportingPlaneOptions) (r SurfaceSupportingPlaneCertificate, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return r, ErrConvergence
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	nonzero := false
	for _, c := range q {
		if c != nil {
			if e := budget.check(c); e != nil {
				return r, e
			}
			nonzero = nonzero || c.Sign() != 0
		}
	}
	if !nonzero {
		return r, ErrDegenerate
	}
	du, dv := 2*p.degreeU, 2*p.degreeV
	nu, nv := du+1, dv+1
	values := make([]*big.Rat, nu*nv)
	for i := range values {
		values[i] = new(big.Rat)
	}
	for i, axes := range [10][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}, {0, 3}, {1, 3}, {2, 3}, {3, 3}} {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		c := q[i]
		if c == nil || c.Sign() == 0 {
			continue
		}
		term, e := tensorHomogeneousBernsteinProduct(p, axes[0], p, axes[1], budget)
		if e != nil {
			return r, e
		}
		for j, t := range term {
			values[j].Add(values[j], t.Mul(t, c))
			if e := budget.check(values[j]); e != nil {
				return r, e
			}
		}
	}
	// Treat the polynomial numerator as the X field of an exact scalar
	// patch. It retains original parameter units and the same zero set.
	d := p.Domain()
	clamped := func(lo, hi float64, n int) []float64 {
		r := make([]float64, n*2)
		for j := range n {
			r[j], r[j+n] = lo, hi
		}
		return r
	}
	m := &SurfacePatch{degreeU: du, degreeV: dv, countU: nu, countV: nv,
		knotsU: clamped(d.UMin, d.UMax, nu), knotsV: clamped(d.VMin, d.VMax, nv),
		net: make([]exactHomogeneous, len(values)), options: SurfacePatchOptions{
			MaxPoles: max(p.options.MaxPoles, len(values)*2), MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		}}
	for j, v := range values {
		m.net[j] = exactHomogeneous{v, new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	}
	if e := m.computeBounds(budget); e != nil {
		return r, e
	}
	return certifySurfaceSupportingEquation(ctx, m, o, budget, func(s *SurfacePatch) ([]*big.Rat, error) {
		r := make([]*big.Rat, len(s.net))
		for j, h := range s.net {
			r[j] = h[0]
		}
		return r, nil
	})
}
