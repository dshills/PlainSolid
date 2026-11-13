package geom

import (
	"context"
	"math/big"
)

// AffineXYChartAlignment is immutable evidence for identical projected
// rectangular domains. TargetOnSource retains exact model coordinates while
// expressing target parameters in the source chart. Only signed axis
// permutations/scales are supported; a shear is deliberately not a rectangle.
type AffineXYChartAlignment struct {
	source, target *SurfacePatch
	mapping        [2][3]*big.Rat
	operations     int
}

func (a *AffineXYChartAlignment) TargetOnSource() *SurfacePatch {
	if a == nil {
		return nil
	}
	return a.target
}
func (a *AffineXYChartAlignment) ExactOperations() int {
	if a == nil {
		return 0
	}
	return a.operations
}
func (a *AffineXYChartAlignment) MapCoefficients() ([2][3]*big.Rat, error) {
	var r [2][3]*big.Rat
	if a == nil || a.target == nil {
		return r, ErrInvalid
	}
	for i := range r {
		for j := range r[i] {
			r[i][j] = new(big.Rat).Set(a.mapping[i][j])
		}
	}
	return r, nil
}

// AlignAffineXYCharts proves target.UV=M*source.UV+c exactly. It requires full
// matching projected domains. Nonrepresentable remapped internal knots return
// ErrUnrepresentable rather than rounding the chart. All control values remain
// rational, including nonbinary diagonal scales and reversals.
func AlignAffineXYCharts(ctx context.Context, source, target *SurfacePatch, o SurfacePatchOptions) (*AffineXYChartAlignment, error) {
	options, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if ctx == nil {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	var inverse [2][2][3]*big.Rat
	for i, p := range []*SurfacePatch{source, target} {
		_, inverse[i], e = normalizeAffineXYGraph(ctx, p, b)
		if e != nil {
			return nil, e
		}
	}
	// Invert source inverse XY map, then compose with target inverse.
	a := inverse[0]
	det := new(big.Rat).Sub(new(big.Rat).Mul(a[0][0], a[1][1]), new(big.Rat).Mul(a[0][1], a[1][0]))
	forward := [2][3]*big.Rat{{new(big.Rat).Quo(a[1][1], det), new(big.Rat).Quo(new(big.Rat).Neg(a[0][1]), det), nil}, {new(big.Rat).Quo(new(big.Rat).Neg(a[1][0]), det), new(big.Rat).Quo(a[0][0], det), nil}}
	for i := 0; i < 2; i++ {
		forward[i][2] = new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(forward[i][0], a[0][2]), new(big.Rat).Mul(forward[i][1], a[1][2])))
	}
	result := &AffineXYChartAlignment{source: source}
	var axes [2]int
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			q := new(big.Rat)
			for k := 0; k < 2; k++ {
				q.Add(q, new(big.Rat).Mul(inverse[1][i][k], forward[k][j]))
			}
			if j == 2 {
				q.Add(q, inverse[1][i][2])
			}
			if e = b.check(q); e != nil {
				return nil, e
			}
			result.mapping[i][j] = q
		}
		axes[i] = -1
		for j := 0; j < 2; j++ {
			if result.mapping[i][j].Sign() != 0 {
				if axes[i] >= 0 {
					return nil, ErrConvergence
				}
				axes[i] = j
			}
		}
		if axes[i] < 0 {
			return nil, ErrDegenerate
		}
	}
	if axes[0] == axes[1] {
		return nil, ErrDegenerate
	}
	sd, td := source.Domain(), target.Domain()
	sr := [2][2]float64{{sd.UMin, sd.UMax}, {sd.VMin, sd.VMax}}
	tr := [2][2]float64{{td.UMin, td.UMax}, {td.VMin, td.VMax}}
	for i, j := range axes {
		for side := 0; side < 2; side++ {
			q := new(big.Rat).Add(new(big.Rat).Mul(result.mapping[i][j], new(big.Rat).SetFloat64(sr[j][side])), result.mapping[i][2])
			ts := side
			if result.mapping[i][j].Sign() < 0 {
				ts = 1 - side
			}
			if q.Cmp(new(big.Rat).SetFloat64(tr[i][ts])) != 0 {
				return nil, ErrDomain
			}
		}
	}
	p := *target
	p.options = options
	p.net = make([]exactHomogeneous, len(target.net))
	if len(p.net) > options.MaxPoles {
		return nil, ErrConvergence
	}
	degrees := [2]int{target.degreeU, target.degreeV}
	counts := [2]int{target.countU, target.countV}
	knots := [2][]float64{target.knotsU, target.knotsV}
	var newKnots [2][]float64
	var newDegree, newCount [2]int
	for i, j := range axes {
		newDegree[j] = degrees[i]
		newCount[j] = counts[i]
		newKnots[j] = make([]float64, len(knots[i]))
		for k, v := range knots[i] {
			q := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(v), result.mapping[i][2]), result.mapping[i][j])
			if e = b.check(q); e != nil {
				return nil, e
			}
			f, e := RepresentedExactScalar(q)
			if e != nil {
				return nil, e
			}
			index := k
			if result.mapping[i][j].Sign() < 0 {
				index = len(knots[i]) - 1 - k
			}
			newKnots[j][index] = f
		}
	}
	p.degreeU, p.degreeV = newDegree[0], newDegree[1]
	p.countU, p.countV = newCount[0], newCount[1]
	p.knotsU, p.knotsV = newKnots[0], newKnots[1]
	for u := 0; u < p.countU; u++ {
		for v := 0; v < p.countV; v++ {
			ij := [2]int{u, v}
			var old [2]int
			for i, j := range axes {
				old[i] = ij[j]
				if result.mapping[i][j].Sign() < 0 {
					old[i] = counts[i] - 1 - old[i]
				}
			}
			p.net[u*p.countV+v] = target.net[old[0]*target.countV+old[1]]
		}
	}
	if e = p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	result.target = &p
	result.operations = b.operations
	return result, nil
}

// AlignedGraphHeightRange bounds source Z minus target Z throughout the common
// projected rectangle. StrictSign is nonzero only for a strict global order;
// zero does not imply coincidence. Exact bounds remain useful after underflow.
type AlignedGraphHeightRange struct {
	Lower, Upper    *big.Rat
	Range           CertifiedScalarRange
	StrictSign      int
	ExactOperations int
}

func (a *AffineXYChartAlignment) HeightRange(ctx context.Context, o GeneralSurfaceHeightOptions) (r AlignedGraphHeightRange, err error) {
	if a == nil || a.source == nil || a.target == nil {
		return r, ErrInvalid
	}
	p, e := NewGeneralSurfaceHeightDifferencePatch(ctx, a.source, a.target, o)
	if e != nil {
		return r, e
	}
	r.ExactOperations = p.operations
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits, operations: p.operations}
	defer func() { r.ExactOperations = budget.operations }()
	for _, h := range p.net {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		q := new(big.Rat).Quo(h[2], h[3])
		if e := budget.check(q); e != nil {
			return r, e
		}
		if r.Lower == nil || q.Cmp(r.Lower) < 0 {
			r.Lower = new(big.Rat).Set(q)
		}
		if r.Upper == nil || q.Cmp(r.Upper) > 0 {
			r.Upper = new(big.Rat).Set(q)
		}
	}
	r.Range = CertifiedScalarRange{Min: rationalFloatBound(r.Lower, true), Max: rationalFloatBound(r.Upper, false)}
	if r.Lower.Sign() > 0 {
		r.StrictSign = 1
	} else if r.Upper.Sign() < 0 {
		r.StrictSign = -1
	}
	return r, nil
}
