package geom

import (
	"context"
	"math/big"
)

// AffineChartIntersectionKind classifies two closed chart rectangles. Coplanar
// identifies coincident supporting planes only; their polygon overlap is not computed.
type AffineChartIntersectionKind uint8

const (
	AffineChartsDisjoint AffineChartIntersectionKind = iota
	AffineChartsCoplanar
	AffineChartsPoint
	AffineChartsSegment
)

// AffineChartIntersectionOptions limits rational elimination; zero selects
// 100000 operations and 16384 coefficient bits.
type AffineChartIntersectionOptions struct{ MaxOperations, MaxCoefficientBits int }

// AffineChartIntersectionEndpoint owns its rational coordinates. Boundary masks
// use bits 1,2,4,8 for UMin, UMax, VMin, VMax respectively.
type AffineChartIntersectionEndpoint struct {
	Point                [3]*big.Rat
	A, B                 [2]*big.Rat
	BoundaryA, BoundaryB uint8
}

// AffineChartIntersection contains exact clipping results. Endpoints are valid
// only for Point or Segment; Complete is false on any resource or input error.
type AffineChartIntersection struct {
	Kind            AffineChartIntersectionKind
	Complete        bool
	Endpoints       [2]AffineChartIntersectionEndpoint
	ExactOperations int
}

// IntersectAffineSurfaceCharts solves the exact affine chart equations and clips
// their line to both closed rectangles. Point results repeat the endpoint.
// No angular or distance tolerance is used, even for nearly parallel planes.
func IntersectAffineSurfaceCharts(ctx context.Context, a, b *AffineSurfaceChart, o AffineChartIntersectionOptions) (r AffineChartIntersection, err error) {
	if ctx == nil || a == nil || b == nil || a.determinant == nil || b.determinant == nil {
		return r, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	check := func(x *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		return budget.check(x)
	}
	var m [3][5]*big.Rat
	for i := 0; i < 3; i++ {
		m[i] = [5]*big.Rat{new(big.Rat).Set(a.u[i]), new(big.Rat).Set(a.v[i]), new(big.Rat).Neg(b.u[i]), new(big.Rat).Neg(b.v[i]), new(big.Rat).Sub(b.origin[i], a.origin[i])}
		for _, x := range m[i] {
			if e := check(x); e != nil {
				return r, e
			}
		}
	}
	rank := 0
	pivots := []int{}
	for col := 0; col < 4 && rank < 3; col++ {
		row := rank
		for row < 3 && m[row][col].Sign() == 0 {
			row++
		}
		if row == 3 {
			continue
		}
		m[rank], m[row] = m[row], m[rank]
		divisor := new(big.Rat).Set(m[rank][col])
		for j := col; j < 5; j++ {
			m[rank][j].Quo(m[rank][j], divisor)
			if e := check(m[rank][j]); e != nil {
				return r, e
			}
		}
		for i := 0; i < 3; i++ {
			if i == rank {
				continue
			}
			factor := new(big.Rat).Set(m[i][col])
			for j := col; j < 5; j++ {
				m[i][j].Sub(m[i][j], new(big.Rat).Mul(factor, m[rank][j]))
				if e := check(m[i][j]); e != nil {
					return r, e
				}
			}
		}
		pivots = append(pivots, col)
		rank++
	}
	if rank < 3 {
		r.Complete = true
		r.Kind = AffineChartsCoplanar
		for i := rank; i < 3; i++ {
			if m[i][4].Sign() != 0 {
				r.Kind = AffineChartsDisjoint
			}
		}
		return r, nil
	}
	free := 0
	for _, p := range pivots {
		if p == free {
			free++
		}
	}
	var origin, direction [4]*big.Rat
	for i := 0; i < 4; i++ {
		origin[i] = new(big.Rat)
		direction[i] = new(big.Rat)
	}
	direction[free].SetInt64(1)
	for i, p := range pivots {
		origin[p].Set(m[i][4])
		direction[p].Neg(m[i][free])
	}
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	for i := 0; i < 4; i++ {
		if direction[i].Sign() == 0 {
			if origin[i].Sign() < 0 || origin[i].Cmp(big.NewRat(1, 1)) > 0 {
				r.Complete = true
				return r, nil
			}
			continue
		}
		x := new(big.Rat).Quo(new(big.Rat).Neg(origin[i]), direction[i])
		y := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(1, 1), origin[i]), direction[i])
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		if e := check(x); e != nil {
			return r, e
		}
		if e := check(y); e != nil {
			return r, e
		}
		if x.Cmp(lo) > 0 {
			lo.Set(x)
		}
		if y.Cmp(hi) < 0 {
			hi.Set(y)
		}
	}
	if lo.Cmp(hi) > 0 {
		r.Complete = true
		return r, nil
	}
	for end, t := range []*big.Rat{lo, hi} {
		var q [4]*big.Rat
		for i := 0; i < 4; i++ {
			q[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(t, direction[i]))
			if e := check(q[i]); e != nil {
				return r, e
			}
		}
		ep := &r.Endpoints[end]
		for axis := 0; axis < 3; axis++ {
			ep.Point[axis] = new(big.Rat).Add(a.origin[axis], new(big.Rat).Add(new(big.Rat).Mul(q[0], a.u[axis]), new(big.Rat).Mul(q[1], a.v[axis])))
			if e := check(ep.Point[axis]); e != nil {
				return r, e
			}
		}
		for chart, c := range []*AffineSurfaceChart{a, b} {
			mins := [2]float64{c.domain.UMin, c.domain.VMin}
			maxs := [2]float64{c.domain.UMax, c.domain.VMax}
			var uv [2]*big.Rat
			var mask uint8
			for i := 0; i < 2; i++ {
				s := q[2*chart+i]
				if s.Sign() == 0 {
					mask |= 1 << uint(2*i)
				}
				if s.Cmp(big.NewRat(1, 1)) == 0 {
					mask |= 1 << uint(2*i+1)
				}
				mn := new(big.Rat).SetFloat64(mins[i])
				width := new(big.Rat).Sub(new(big.Rat).SetFloat64(maxs[i]), mn)
				uv[i] = new(big.Rat).Add(mn, new(big.Rat).Mul(width, s))
				if e := check(uv[i]); e != nil {
					return r, e
				}
			}
			if chart == 0 {
				ep.A = uv
				ep.BoundaryA = mask
			} else {
				ep.B = uv
				ep.BoundaryB = mask
			}
		}
	}
	r.Kind = AffineChartsSegment
	if lo.Cmp(hi) == 0 {
		r.Kind = AffineChartsPoint
	}
	r.Complete = true
	return r, nil
}
