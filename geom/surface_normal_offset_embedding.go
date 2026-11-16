package geom

import (
	"context"
	"math/big"
)

// NormalOffsetProjection selects two distinct model coordinates, in the
// order that corresponds to positive U and V on the original chart.
// Reversed negates the corresponding model coordinate before the proof.
type NormalOffsetProjection struct {
	Axes     [2]int
	Reversed [2]bool
}

// CertifyProjectedNormalOffsetEmbedding proves the normal offset of a
// single smooth rational Bezier patch is an injective immersion through
// the given projection, separately at every fixed displacement from zero
// through distance. It bounds the symmetric part of the projected
// Jacobian uniformly on the full original rectangle. Convexity of that
// domain then proves that distinct UV cannot map to the same point.
// It does not certify the swept volume, neighboring faces, or creases.
// ErrConvergence retains all parameter cells where the proof is unresolved.
func (p *SurfacePatch) CertifyProjectedNormalOffsetEmbedding(ctx context.Context, distance *big.Rat,
	projection NormalOffsetProjection, o NormalOffsetCoverageOptions) (NormalOffsetCoverage, error) {
	x, y := projection.Axes[0], projection.Axes[1]
	if ctx == nil || p == nil || len(p.net) == 0 || distance == nil || x < 0 || x > 2 || y < 0 || y > 2 || x == y {
		return NormalOffsetCoverage{}, ErrInvalid
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return NormalOffsetCoverage{Unresolved: []ParameterRectangle{p.Domain()}}, ErrDomain
	}
	return p.certifyNormalOffsetCells(ctx, distance, o, func(ctx context.Context, p *SurfacePatch, d *big.Rat,
		b *polynomialBudget) (bool, error) {
		return projectedOffsetCellMonotone(ctx, p, d, projection, b)
	})
}

func projectedOffsetCellMonotone(ctx context.Context, p *SurfacePatch, distance *big.Rat, projection NormalOffsetProjection,
	b *polynomialBudget) (bool, error) {
	d, err := offsetCellJacobianBounds(ctx, p, b)
	if err != nil || d == nil {
		return false, err
	}
	a := &d.arithmetic
	zero := offsetZeroInterval()
	shift := exactInterval{zero.lo, distance}
	if distance.Sign() < 0 {
		shift.lo, shift.hi = shift.hi, shift.lo
	}
	var matrix [2][2]exactInterval
	for row, i := range projection.Axes {
		du, dv := d.normalDerivative(i, 0), d.normalDerivative(i, 1)
		matrix[row][0] = a.add(d.tangent[0][i], a.mul(shift, du))
		matrix[row][1] = a.add(d.tangent[1][i], a.mul(shift, dv))
		if projection.Reversed[row] {
			matrix[row][0] = a.sub(zero, matrix[row][0])
			matrix[row][1] = a.sub(zero, matrix[row][1])
		}
	}
	if a.err != nil || matrix[0][0].lo.Sign() <= 0 || matrix[1][1].lo.Sign() <= 0 {
		return false, a.err
	}
	side := a.add(matrix[0][1], matrix[1][0])
	four := big.NewRat(4, 1)
	det := a.sub(a.mul(exactInterval{four, four}, a.mul(matrix[0][0], matrix[1][1])), a.square(side))
	return a.err == nil && det.lo.Sign() > 0, a.err
}

type offsetCellJacobian struct {
	arithmetic               offsetIntervalArithmetic
	tangent, normalPartials  [2][3]exactInterval
	point, normal            [3]exactInterval
	normalDot                [2]exactInterval
	length, length2, length3 exactInterval
}

func offsetCellJacobianBounds(ctx context.Context, p *SurfacePatch, b *polynomialBudget) (*offsetCellJacobian, error) {
	jet, err := offsetCartesianRanges(ctx, p, b)
	if err != nil {
		return nil, err
	}
	a := offsetIntervalArithmetic{b: b}
	u, v := jet[1][0], jet[0][1]
	n := a.cross(u, v)
	nu1, nu2 := a.cross(jet[2][0], v), a.cross(u, jet[1][1])
	nv1, nv2 := a.cross(jet[1][1], v), a.cross(u, jet[0][2])
	scale := new(big.Rat)
	for _, span := range n {
		for _, x := range []*big.Rat{span.lo, span.hi} {
			if abs := new(big.Rat).Abs(x); abs.Cmp(scale) > 0 {
				scale = abs
			}
		}
	}
	if a.err != nil || scale.Sign() == 0 {
		return nil, a.err
	}
	// Scale by a fixed positive value over this cell to avoid losing
	// small but nonzero normals in the square-root enclosure.
	s := exactInterval{scale, scale}
	var nu, nv [3]exactInterval
	for i := range n {
		n[i] = a.div(n[i], s)
		nu[i] = a.div(a.add(nu1[i], nu2[i]), s)
		nv[i] = a.div(a.add(nv1[i], nv2[i]), s)
	}
	length2 := a.norm2(n)
	if a.err != nil || length2.lo.Sign() <= 0 {
		return nil, a.err
	}
	lo, _, err := rationalSqrtBounds(length2.lo, 96, b)
	if err != nil {
		return nil, err
	}
	_, hi, err := rationalSqrtBounds(length2.hi, 96, b)
	if err != nil || lo.Sign() <= 0 {
		return nil, err
	}
	length, err := boundedExactInterval(lo, hi, b)
	if err != nil {
		return nil, err
	}
	length3 := a.mul(length2, length)
	nuDot, nvDot := a.dot(n, nu), a.dot(n, nv)
	if a.err != nil {
		return nil, a.err
	}
	return &offsetCellJacobian{arithmetic: a, tangent: [2][3]exactInterval{u, v}, point: jet[0][0], normal: n,
		normalPartials: [2][3]exactInterval{nu, nv}, normalDot: [2]exactInterval{nuDot, nvDot},
		length: length, length2: length2, length3: length3}, nil
}

func (d *offsetCellJacobian) normalDerivative(i, parameter int) exactInterval {
	a := &d.arithmetic
	// Differentiate the normalized vector:
	// n' = (N' |N|² - N (N·N')) / |N|³.
	return a.div(a.sub(a.mul(d.normalPartials[parameter][i], d.length2), a.mul(d.normal[i], d.normalDot[parameter])), d.length3)
}
