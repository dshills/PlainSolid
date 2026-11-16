package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactPeriodicSplineFundamental assembles the exact physical tensor on
// integer working knot intervals. Only position closure is asserted for
// its working seams: unequal knot widths can rescale the first derivatives.
// Coordinates maps each working axis back to the original physical one.
type ExactPeriodicSplineFundamental struct {
	Wrapped         *WrappedSurfacePatch
	Coordinates     [2]ExactSplineAxisCoordinates
	Seam            SurfaceSeamCertificate
	ExactOperations int
}

// ExactSplineAxisCoordinates maps a fundamental interval one occupied
// knot span at a time. Working intervals [i,i+1] have the corresponding
// physical bounds and original span IDs. It does not wrap the query.
type ExactSplineAxisCoordinates struct {
	breaks []*big.Rat
	spans  []int
	bits   int
}

// Breaks returns caller-owned physical knot positions and the numerical
// knot-slot index for each occupied span, skipping repeated empty slots.
func (m ExactSplineAxisCoordinates) Breaks() ([]*big.Rat, []int) {
	b := make([]*big.Rat, len(m.breaks))
	for i, q := range m.breaks {
		b[i] = new(big.Rat).Set(q)
	}
	return b, append([]int(nil), m.spans...)
}

// ToPhysical maps a represented working coordinate to the exact original
// physical coordinate. The last endpoint belongs to the last span.
func (m ExactSplineAxisCoordinates) ToPhysical(t *big.Rat) (*big.Rat, error) {
	if t == nil || len(m.spans) == 0 {
		return nil, ErrInvalid
	}
	if t.Num().BitLen() > m.bits || t.Denom().BitLen() > m.bits {
		return nil, ErrConvergence
	}
	n := int64(len(m.spans))
	if t.Sign() < 0 || t.Cmp(big.NewRat(n, 1)) > 0 {
		return nil, ErrDomain
	}
	i := min(new(big.Int).Div(t.Num(), t.Denom()).Int64(), n-1)
	x := new(big.Rat).Sub(t, big.NewRat(i, 1))
	x.Mul(x, new(big.Rat).Sub(m.breaks[i+1], m.breaks[i])).Add(x, m.breaks[i])
	return m.checked(x)
}

// ToWorking maps a physical coordinate inside the fundamental interval
// onto the corresponding exact integer-index working coordinate.
func (m ExactSplineAxisCoordinates) ToWorking(t *big.Rat) (*big.Rat, error) {
	if t == nil || len(m.spans) == 0 {
		return nil, ErrInvalid
	}
	if t.Num().BitLen() > m.bits || t.Denom().BitLen() > m.bits {
		return nil, ErrConvergence
	}
	n := len(m.spans)
	if t.Cmp(m.breaks[0]) < 0 || t.Cmp(m.breaks[n]) > 0 {
		return nil, ErrDomain
	}
	i := min(sort.Search(n, func(i int) bool { return m.breaks[i+1].Cmp(t) > 0 }), n-1)
	x := new(big.Rat).Quo(new(big.Rat).Sub(t, m.breaks[i]), new(big.Rat).Sub(m.breaks[i+1], m.breaks[i]))
	x.Add(x, big.NewRat(int64(i), 1))
	return m.checked(x)
}

func (m ExactSplineAxisCoordinates) checked(q *big.Rat) (*big.Rat, error) {
	if q.Num().BitLen() > m.bits || q.Denom().BitLen() > m.bits {
		return nil, ErrConvergence
	}
	return q, nil
}

// ExactFundamentalPatch joins the original homogeneous Bézier cells on a
// lossless rational coordinate map and proves position closure on every
// cyclic seam. It refuses discontinuous ordinary axes instead of welding
// their distinct one-sided limits. No working knot rounding changes the law.
func (s *ExactPeriodicSplineSurface) ExactFundamentalPatch(ctx context.Context, o PeriodicSurfaceBezierOptions) (result ExactPeriodicSplineFundamental, err error) {
	if ctx == nil || s == nil || len(s.net) == 0 || o.MaxCells < 0 {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.SurfacePatchOptions)
	if err != nil {
		return result, err
	}
	var domain [2]ExactParameterInterval
	var wrap [2]bool
	for direction, a := range s.axis {
		wrap[direction] = a.period != nil
		first := 0
		if !wrap[direction] {
			first = a.degree
		}
		domain[direction] = ExactParameterInterval{Lower: a.knots[first], Upper: a.knots[a.count]}
	}
	cells, work, err := s.ExactBezierCells(ctx, domain[0], domain[1], o)
	b := &polynomialBudget{operations: work, maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if err != nil {
		return result, err
	}
	// Both working axes use integer intervals. Wrap flags below still
	// follow the original source; reindexing does not make an ordinary
	// axis periodic.
	p, err := assemblePeriodicFundamental(ctx, cells, [2]bool{true, true}, limits, b)
	if err != nil {
		return result, err
	}
	remaining := limits.MaxOperations - b.operations
	if remaining < 1 {
		return result, ErrConvergence
	}
	w, seam, err := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{
		WrapU: wrap[0], WrapV: wrap[1], MaxPoles: limits.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	b.operations += seam.ExactOperations
	if err != nil {
		return result, err
	}
	w.options.MaxOperations = limits.MaxOperations
	var coords [2]ExactSplineAxisCoordinates
	for i := range coords {
		coords[i].bits = limits.MaxCoefficientBits
	}
	for _, c := range cells {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		// Cells are U-major. Only the row/column on the original lower
		// boundary supplies the independently owned coordinates.
		if c.V.Domain.Lower.Cmp(domain[1].Lower) == 0 {
			coords[0].appendSpan(c.U)
		}
		if c.U.Domain.Lower.Cmp(domain[0].Lower) == 0 {
			coords[1].appendSpan(c.V)
		}
	}
	result.Wrapped, result.Coordinates, result.Seam = w, coords, seam
	return result, nil
}

func (m *ExactSplineAxisCoordinates) appendSpan(s ExactSurfaceKnotSpan) {
	if len(m.breaks) == 0 {
		m.breaks = append(m.breaks, new(big.Rat).Set(s.Domain.Lower))
	}
	m.breaks = append(m.breaks, new(big.Rat).Set(s.Domain.Upper))
	m.spans = append(m.spans, s.Span)
}
