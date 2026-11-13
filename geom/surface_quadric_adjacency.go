package geom

import (
	"context"
	"errors"
	"math/big"
)

// SurfaceQuadricAdjacencyCertificate records an exact separating quadric and
// the supporting sign/zero proofs for both original parameter rectangles.
type SurfaceQuadricAdjacencyCertificate struct {
	Complete, Separated, ReverseB             bool
	Quadric                                   ExactQuadric
	A, B                                      SurfaceSupportingPlaneCertificate
	Candidates, VisitedCells, ExactOperations int
}

// CertifyQuadricAdjacentSurfaceSeparation searches selected quadrics through
// a common exact full edge. It first verifies that the edge curves agree,
// then proves the two full patches lie on opposite strict sides of the
// same quadric away from that edge. Candidate generation currently targets
// a quadratic vertical warp of a straight edge; acceptance is exact.
func CertifyQuadricAdjacentSurfaceSeparation(ctx context.Context, a, b *SurfacePatch, edgeA, edgeB SurfaceEdge, o SurfaceAdjacencyOptions) (r SurfaceQuadricAdjacencyCertificate, err error) {
	if ctx == nil || a == nil || b == nil || edgeA > SurfaceEdgeVMax || edgeB > SurfaceEdgeVMax {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 16
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 0 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	for _, p := range []*SurfacePatch{a, b} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return r, ErrConvergence
		}
	}
	edge := [2]SurfaceEdge{edgeA, edgeB}
	var curves [2]*SurfaceIsoCurve
	for i, p := range []*SurfacePatch{a, b} {
		d := p.Domain()
		t := [4]float64{d.UMin, d.UMax, d.VMin, d.VMax}[edge[i]]
		left := o.MaxOperations - r.ExactOperations
		if left < 1 {
			return r, ErrConvergence
		}
		q, e := p.IsoCurve(int(edge[i])/2, t, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		r.ExactOperations += q.operations
		curves[i] = q
	}
	match := false
	for _, reverse := range []bool{false, true} {
		left := o.MaxOperations - r.ExactOperations
		if left < 1 {
			return r, ErrConvergence
		}
		q, e := CompareExactCurvesContext(ctx, curves[0], curves[1], ExactCurveComparisonOptions{
			NormalizeDomains: true, ReverseB: reverse, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		r.ExactOperations += q.ExactOperations
		if e != nil {
			return r, e
		}
		if q.Complete && q.Equal {
			r.ReverseB, match = reverse, true
			break
		}
	}
	if !match {
		return r, ErrConvergence
	}
	// Fit z = c*t² + l*t + d for each nonconstant XY covector t.
	// Add a linear separator through the projected edge; this provides
	// a family of quadrics sharing that edge, even on an oblique patch.
	var samples [3][3]*big.Rat
	lo, hi := curves[0].Domain()
	budget := &polynomialBudget{maxOperations: o.MaxOperations - r.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	for i, x := range []float64{lo, lo/2 + hi/2, hi} {
		samples[i], err = exactPatchPointAt(curves[0].patch, x, 0, budget)
		if err != nil {
			return r, err
		}
	}
	dx := new(big.Rat).Sub(samples[2][0], samples[0][0])
	dy := new(big.Rat).Sub(samples[2][1], samples[0][1])
	if dx.Sign() == 0 && dy.Sign() == 0 {
		return r, ErrConvergence
	}
	// Normalize so the largest normal component has magnitude one.
	scale := new(big.Rat).Abs(dx)
	if y := new(big.Rat).Abs(dy); y.Cmp(scale) > 0 {
		scale = y
	}
	nx := new(big.Rat).Quo(dy, scale)
	ny := new(big.Rat).Quo(new(big.Rat).Neg(dx), scale)
	d0 := new(big.Rat).Add(new(big.Rat).Mul(nx, samples[0][0]), new(big.Rat).Mul(ny, samples[0][1]))
	for _, d := range [][2]int64{{1, -1}, {1, 1}, {1, 0}, {0, 1}} {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		m, n := big.NewRat(d[0], 1), big.NewRat(d[1], 1)
		var matrix [3][3]*big.Rat
		for i, p := range samples {
			t := new(big.Rat).Add(new(big.Rat).Mul(m, p[0]), new(big.Rat).Mul(n, p[1]))
			matrix[i] = [3]*big.Rat{new(big.Rat).Mul(t, t), t, big.NewRat(1, 1)}
		}
		budget.maxOperations = o.MaxOperations - r.ExactOperations
		if budget.maxOperations <= budget.operations {
			return r, ErrConvergence
		}
		inverse, ok, e := inverseExact3(matrix, budget)
		r.ExactOperations += budget.operations
		budget.operations = 0
		if e != nil {
			return r, e
		}
		if !ok {
			continue
		}
		var f [3]*big.Rat
		for i := range f {
			f[i] = new(big.Rat)
			for j, z := range samples {
				f[i].Add(f[i], new(big.Rat).Mul(inverse[i][j], z[2]))
			}
		}
		if f[0].Sign() == 0 {
			continue
		}
		for _, slope := range []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(-1, 2), big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(2, 1), big.NewRat(-2, 1)} {
			if r.Candidates >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.Candidates++
			mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
			sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
			neg := func(a *big.Rat) *big.Rat { return new(big.Rat).Neg(a) }
			q := ExactQuadric{neg(mul(f[0], mul(m, m))), neg(mul(f[0], mul(n, n))), nil,
				neg(mul(big.NewRat(2, 1), mul(f[0], mul(m, n)))), nil, nil,
				sub(mul(slope, nx), mul(f[1], m)), sub(mul(slope, ny), mul(f[1], n)), big.NewRat(1, 1),
				sub(neg(f[2]), mul(slope, d0))}
			var c [2]SurfaceSupportingPlaneCertificate
			for i, p := range []*SurfacePatch{a, b} {
				left := o.MaxOperations - r.ExactOperations
				if left < 1 {
					return r, ErrConvergence
				}
				cells := o.MaxCells
				if cells > 0 {
					cells -= r.VisitedCells
					if cells < 1 {
						return r, ErrConvergence
					}
				}
				x, e := CertifySurfaceSupportingQuadric(ctx, p, q, SurfaceSupportingPlaneOptions{
					MaxDepth: o.MaxDepth, MaxCells: cells, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
				})
				r.ExactOperations += x.ExactOperations
				r.VisitedCells += x.VisitedCells
				if e != nil && !errors.Is(e, ErrConvergence) {
					return r, e
				}
				if e != nil || x.Coincident || !x.Complete || x.SideMask != 1<<edge[i] || x.CornerMask != 0 {
					break
				}
				c[i] = x
			}
			if c[1].Complete && c[0].HalfspaceSign == -c[1].HalfspaceSign {
				r.Complete, r.Separated, r.A, r.B = true, true, c[0], c[1]
				r.Quadric = q
				return r, nil
			}
		}
	}
	return r, ErrConvergence
}
