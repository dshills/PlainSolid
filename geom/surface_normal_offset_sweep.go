package geom

import (
	"context"
	"math/big"
)

// NormalOffsetSweepProjection is a signed permutation of model
// coordinates, in the order of the original U, V and signed displacement.
type NormalOffsetSweepProjection struct {
	Axes     [3]int
	Reversed [3]bool
}

// CertifyNormalOffsetSweep proves that S(u,v)+t*n(u,v) is an injective
// immersion of the entire original rectangle times [lower,upper]. It
// treats different displacement layers together, rather than proving
// each distance independently. The interval must have positive width.
// Only a single smooth rational Bezier patch is supported; this does
// not check other patches or create boundary topology.
func (p *SurfacePatch) CertifyNormalOffsetSweep(ctx context.Context, lower, upper *big.Rat,
	projection NormalOffsetSweepProjection, o NormalOffsetCoverageOptions) (NormalOffsetCoverage, error) {
	if ctx == nil || p == nil || len(p.net) == 0 || lower == nil || upper == nil || lower.Cmp(upper) >= 0 {
		return NormalOffsetCoverage{}, ErrInvalid
	}
	var mask uint
	for _, axis := range projection.Axes {
		if axis < 0 || axis > 2 || mask&(1<<axis) != 0 {
			return NormalOffsetCoverage{}, ErrInvalid
		}
		mask |= 1 << axis
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return NormalOffsetCoverage{Unresolved: []ParameterRectangle{p.Domain()}}, ErrDomain
	}
	return p.certifyNormalOffsetCells(ctx, upper, o, func(ctx context.Context, cell *SurfacePatch, upper *big.Rat,
		b *polynomialBudget) (bool, error) {
		if err := b.check(lower); err != nil {
			return false, err
		}
		return offsetSweepCell(ctx, cell, lower, upper, projection, b)
	})
}

func offsetSweepCell(ctx context.Context, p *SurfacePatch, lower, upper *big.Rat, projection NormalOffsetSweepProjection,
	b *polynomialBudget) (bool, error) {
	d, err := offsetCellJacobianBounds(ctx, p, b)
	if err != nil || d == nil {
		return false, err
	}
	a := &d.arithmetic
	var normals, nu, nv [3]exactInterval
	for row, model := range projection.Axes {
		normals[row] = a.div(d.normal[model], d.length)
		nu[row], nv[row] = d.normalDerivative(model, 0), d.normalDerivative(model, 1)
	}
	zero := offsetZeroInterval()
	// At fixed UV, the Jacobian is affine in displacement. Strict
	// positive definiteness of its symmetric part at both ends implies
	// it everywhere between them. Admissible cells cover a convex
	// three-dimensional source, so integrating along a parameter line
	// proves distinct parameters have distinct projected images.
	for _, t := range []*big.Rat{lower, upper} {
		shift := exactInterval{t, t}
		var j [3][3]exactInterval
		for row, model := range projection.Axes {
			j[row][0] = a.add(d.tangent[0][model], a.mul(shift, nu[row]))
			j[row][1] = a.add(d.tangent[1][model], a.mul(shift, nv[row]))
			j[row][2] = normals[row]
			if projection.Reversed[row] {
				for column := range 3 {
					j[row][column] = a.sub(zero, j[row][column])
				}
			}
		}
		if a.err != nil || !positiveSweepJacobian(a, j) {
			return false, a.err
		}
	}
	return true, nil
}

func positiveSweepJacobian(a *offsetIntervalArithmetic, j [3][3]exactInterval) bool {
	// The leading principal minors of the symmetric part must all be
	// positive. Test their exact directed interval lower endpoints.
	a0, a1, a2 := j[0][0], j[1][1], j[2][2]
	if a0.lo.Sign() <= 0 || a1.lo.Sign() <= 0 || a2.lo.Sign() <= 0 {
		return false
	}
	half := big.NewRat(1, 2)
	h := exactInterval{half, half}
	b, c, d := a.mul(h, a.add(j[0][1], j[1][0])), a.mul(h, a.add(j[0][2], j[2][0])),
		a.mul(h, a.add(j[1][2], j[2][1]))
	minor := a.sub(a.mul(a0, a1), a.square(b))
	if minor.lo.Sign() <= 0 {
		return false
	}
	cross := a.mul(a.mul(b, c), d)
	det := a.add(a.mul(a.mul(a0, a1), a2), a.add(cross, cross))
	det = a.sub(a.sub(a.sub(det, a.mul(a0, a.square(d))), a.mul(a1, a.square(c))), a.mul(a2, a.square(b)))
	return a.err == nil && det.lo.Sign() > 0
}
