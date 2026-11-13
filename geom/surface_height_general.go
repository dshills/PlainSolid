package geom

import (
	"context"
	"math/big"
	"sort"
)

// GeneralSurfaceHeightOptions bounds union refinement and exact tensor products.
// Zero limits select 4096 cells/poles, 8M operations and 16384 coefficient bits.
type GeneralSurfaceHeightOptions struct{ MaxCells, MaxPoles, MaxOperations, MaxCoefficientBits int }

// NewGeneralSurfaceHeightDifferencePatch constructs exact (X,Y,ZA-ZB) for
// same-domain patches with provably identical rational X/Y parameter maps.
// Degrees, knot partitions and positive weight fields may differ. Each union
// cell uses positive product weight WA*WB, degree sums, and exact homogeneous
// numerator products; shared boundaries are joined with degree-multiplicity C0
// knots. The resulting function retains any higher smoothness algebraically,
// although the knot representation does not promise it. XY mismatch is
// ErrInvalid, resource exhaustion ErrConvergence. The existing fast constructor
// for identical bases remains separate. No float pole reconstruction occurs.
func NewGeneralSurfaceHeightDifferencePatch(ctx context.Context, a, b *SurfacePatch, o GeneralSurfaceHeightOptions) (*SurfacePatch, error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if a.Domain() != b.Domain() {
		return nil, ErrDomain
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells < 1 || o.MaxPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	union := func(x, y []float64) []float64 {
		values := append(append([]float64(nil), x...), y...)
		sort.Float64s(values)
		unique := values[:0]
		for _, v := range values {
			if len(unique) == 0 || unique[len(unique)-1] != v {
				unique = append(unique, v)
			}
		}
		return unique
	}
	us, vs := union(a.knotsU, b.knotsU), union(a.knotsV, b.knotsV)
	du, dv := a.degreeU+b.degreeU, a.degreeV+b.degreeV
	if len(us)-1 > o.MaxCells/(len(vs)-1) || (len(us)-1) > (o.MaxPoles-1)/du || (len(vs)-1) > (o.MaxPoles-1)/dv {
		return nil, ErrConvergence
	}
	nu, nv := (len(us)-1)*du+1, (len(vs)-1)*dv+1
	if nu > o.MaxPoles/nv {
		return nil, ErrConvergence
	}
	net := make([]exactHomogeneous, nu*nv)
	for i := 0; i < len(us)-1; i++ {
		for j := 0; j < len(vs)-1; j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			rectangle := ParameterRectangle{UMin: us[i], UMax: us[i+1], VMin: vs[j], VMax: vs[j+1]}
			ac, e := trimPatchBudget(a, rectangle, budget)
			if e != nil {
				return nil, e
			}
			bc, e := trimPatchBudget(b, rectangle, budget)
			if e != nil {
				return nil, e
			}
			if ac.countU != ac.degreeU+1 || ac.countV != ac.degreeV+1 || bc.countU != bc.degreeU+1 || bc.countV != bc.degreeV+1 {
				return nil, ErrInvalid
			}
			cell := make([]exactHomogeneous, (du+1)*(dv+1))
			weight, e := tensorHomogeneousBernsteinProduct(ac, 3, bc, 3, budget)
			if e != nil {
				return nil, e
			}
			for k, w := range weight {
				if w.Sign() <= 0 {
					return nil, ErrInvalid
				}
				cell[k][3] = w
			}
			for axis := 0; axis < 3; axis++ {
				left, e := tensorHomogeneousBernsteinProduct(ac, axis, bc, 3, budget)
				if e != nil {
					return nil, e
				}
				right, e := tensorHomogeneousBernsteinProduct(ac, 3, bc, axis, budget)
				if e != nil {
					return nil, e
				}
				for k := range cell {
					if axis < 2 {
						if left[k].Cmp(right[k]) != 0 {
							return nil, ErrInvalid
						}
						cell[k][axis] = left[k]
					} else {
						cell[k][axis] = new(big.Rat).Sub(left[k], right[k])
						if e := budget.check(cell[k][axis]); e != nil {
							return nil, e
						}
					}
				}
			}
			for u := 0; u <= du; u++ {
				for v := 0; v <= dv; v++ {
					index := (i*du+u)*nv + j*dv + v
					h := cell[u*(dv+1)+v]
					if net[index][3] != nil {
						for axis := 0; axis < 4; axis++ {
							if net[index][axis].Cmp(h[axis]) != 0 {
								return nil, ErrConvergence
							}
						}
					} else {
						net[index] = h
					}
				}
			}
		}
	}
	knots := func(breaks []float64, degree int) []float64 {
		var result []float64
		for i, k := range breaks {
			count := degree
			if i == 0 || i == len(breaks)-1 {
				count++
			}
			for j := 0; j < count; j++ {
				result = append(result, k)
			}
		}
		return result
	}
	result := &SurfacePatch{degreeU: du, degreeV: dv, countU: nu, countV: nv, knotsU: knots(us, du), knotsV: knots(vs, dv), net: net, options: SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}}
	if e := result.computeBounds(budget); e != nil {
		return nil, e
	}
	result.operations = budget.operations
	return result, nil
}

func tensorHomogeneousBernsteinProduct(a *SurfacePatch, axisA int, b *SurfacePatch, axisB int, budget *polynomialBudget) ([]*big.Rat, error) {
	factors := func(p, q int) ([][]*big.Rat, error) {
		r := make([][]*big.Rat, p+1)
		for i := 0; i <= p; i++ {
			r[i] = make([]*big.Rat, q+1)
			for j := 0; j <= q; j++ {
				numerator := new(big.Int).Mul(new(big.Int).Binomial(int64(p), int64(i)), new(big.Int).Binomial(int64(q), int64(j)))
				denominator := new(big.Int).Binomial(int64(p+q), int64(i+j))
				r[i][j] = new(big.Rat).SetFrac(numerator, denominator)
				if e := budget.check(r[i][j]); e != nil {
					return nil, e
				}
			}
		}
		return r, nil
	}
	fu, e := factors(a.degreeU, b.degreeU)
	if e != nil {
		return nil, e
	}
	fv, e := factors(a.degreeV, b.degreeV)
	if e != nil {
		return nil, e
	}
	width := a.degreeV + b.degreeV + 1
	result := make([]*big.Rat, (a.degreeU+b.degreeU+1)*width)
	for i := range result {
		result[i] = new(big.Rat)
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			for k := 0; k < b.countU; k++ {
				for l := 0; l < b.countV; l++ {
					value := new(big.Rat).Mul(a.net[i*a.countV+j][axisA], b.net[k*b.countV+l][axisB])
					value.Mul(value, fu[i][k])
					value.Mul(value, fv[j][l])
					index := (i+k)*width + j + l
					result[index].Add(result[index], value)
					if e := budget.check(result[index]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return result, nil
}
