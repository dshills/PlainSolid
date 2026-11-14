package geom

import (
	"context"
	"math/big"
)

// SplineGraphRadialContours proves a complete finite family of strictly nested
// regular intersection curves on two identity-XY graphs. Their XY projections
// are circles; the spatial curves retain the original graphs' heights.
// The proof is immutable and cannot be supplied as unchecked discovery input.
type SplineGraphRadialContours struct {
	center           [2]*big.Rat
	radii            []*big.Rat
	heightScale      *big.Rat // optional exact polynomial height difference, not just numerator
	side, operations int
}

func (r *SplineGraphRadialContours) Center() [2]*big.Rat {
	if r == nil || r.center[0] == nil || r.center[1] == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(r.center[0]), new(big.Rat).Set(r.center[1])}
}
func (r *SplineGraphRadialContours) Radii() []*big.Rat {
	if r == nil || r.center[0] == nil || r.center[1] == nil {
		return nil
	}
	out := make([]*big.Rat, len(r.radii))
	for i, q := range r.radii {
		out[i] = new(big.Rat).Set(q)
	}
	return out
}

// BandSigns returns sign(ZA-ZB) from the central disk through the exterior.
func (r *SplineGraphRadialContours) BandSigns() []int {
	if r == nil || r.center[0] == nil || r.center[1] == nil {
		return nil
	}
	out := make([]int, len(r.radii)+1)
	s := r.side
	if len(r.radii)%2 != 0 {
		s = -s
	}
	for i := range out {
		out[i] = s
		s = -s
	}
	return out
}
func (r *SplineGraphRadialContours) ExactOperations() int {
	if r == nil || r.center[0] == nil || r.center[1] == nil {
		return 0
	}
	return r.operations
}
func (r *SplineGraphRadialContours) MarshalJSON() ([]byte, error) { return nil, ErrUnrepresentable }

// CertifySplineGraphRadialContours independently proves the supplied 1..4 radii
// by a whole homogeneous numerator coefficient identity. Both original charts
// must have identity XY on the unit square and one span, degree at most eight.
// A positive denominator, nonzero scalar residual, distinct positive radii and
// strict domain containment prove that there are exactly these regular curves.
// Other terms, missing roots, repeated roots, boundaries and exhausted budgets
// return no certificate. The supplied radii are not treated as discovery proof.
func CertifySplineGraphRadialContours(ctx context.Context, a, c *SurfacePatch, center [2]*big.Rat, radii []*big.Rat, o GraphPointContactOptions) (*SplineGraphRadialContours, error) {
	if ctx == nil || a == nil || c == nil || center[0] == nil || center[1] == nil || len(radii) < 1 || len(radii) > 4 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, q := range center {
		if e := b.check(q); e != nil {
			return nil, e
		}
	}
	for i, q := range radii {
		if q == nil || q.Sign() <= 0 {
			return nil, ErrInvalid
		}
		if e := b.check(q); e != nil {
			return nil, e
		}
		if i > 0 && q.Cmp(radii[i-1]) <= 0 {
			return nil, ErrInvalid
		}
		for _, x := range center {
			lo, hi := new(big.Rat).Sub(x, q), new(big.Rat).Add(x, q)
			if e := b.check(lo); e != nil {
				return nil, e
			}
			if e := b.check(hi); e != nil {
				return nil, e
			}
			if lo.Sign() <= 0 || hi.Cmp(big.NewRat(1, 1)) >= 0 {
				return nil, ErrConvergence
			}
		}
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.Domain() != (ParameterRectangle{0, 1, 0, 1}) || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 8 || p.degreeV > 8 {
			return nil, ErrConvergence
		}
		remaining := b.maxOperations - b.operations
		if remaining <= 0 {
			return nil, ErrConvergence
		}
		g, work, e := newAffineXYGraphWithWork(ctx, p, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += work
		if e != nil {
			return nil, e
		}
		for i := 0; i < 2; i++ {
			for j := 0; j < 3; j++ {
				want := new(big.Rat)
				if i == j {
					want.SetInt64(1)
				}
				if g.inverse[i][j].Cmp(want) != 0 {
					return nil, ErrConvergence
				}
			}
		}
	}
	p, e := orderingDifference(ctx, a, c, 1, b)
	if e != nil {
		return nil, e
	}
	if p.Domain() != (ParameterRectangle{0, 1, 0, 1}) || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 16 || p.degreeV > 16 {
		return nil, ErrConvergence
	}
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		rows[i], e = bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
	}
	coeff := map[[2]int]*big.Rat{}
	for j := 0; j < p.countV; j++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		column, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		for i, q := range column {
			coeff[[2]int{i, j}] = q
		}
	}
	get := func(i, j int) *big.Rat {
		if q := coeff[[2]int{i, j}]; q != nil {
			return q
		}
		return new(big.Rat)
	}
	scale := get(2*len(radii), 0)
	if scale.Sign() == 0 {
		return nil, ErrConvergence
	}
	expected := map[[2]int]*big.Rat{{0, 0}: big.NewRat(1, 1)}
	for _, radius := range radii {
		constant := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(center[0], center[0]), new(big.Rat).Mul(center[1], center[1])), new(big.Rat).Mul(radius, radius))
		factor := map[[2]int]*big.Rat{{0, 0}: constant, {1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), center[0]), {0, 1}: new(big.Rat).Mul(big.NewRat(-2, 1), center[1]), {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1)}
		next := map[[2]int]*big.Rat{}
		for i, x := range expected {
			for j, y := range factor {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				v := new(big.Rat).Mul(x, y)
				if e := b.check(v); e != nil {
					return nil, e
				}
				key := [2]int{i[0] + j[0], i[1] + j[1]}
				if next[key] == nil {
					next[key] = new(big.Rat)
				}
				next[key].Add(next[key], v)
				if e := b.check(next[key]); e != nil {
					return nil, e
				}
			}
		}
		expected = next
	}
	for i := 0; i < max(p.countU, 2*len(radii)+1); i++ {
		for j := 0; j < max(p.countV, 2*len(radii)+1); j++ {
			q := new(big.Rat)
			if v := expected[[2]int{i, j}]; v != nil {
				q.Mul(v, scale)
			}
			if e := b.check(q); e != nil {
				return nil, e
			}
			if q.Cmp(get(i, j)) != 0 {
				return nil, ErrConvergence
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r := &SplineGraphRadialContours{center: [2]*big.Rat{new(big.Rat).Set(center[0]), new(big.Rat).Set(center[1])}, side: scale.Sign(), operations: b.operations}
	for _, q := range radii {
		r.radii = append(r.radii, new(big.Rat).Set(q))
	}
	constantWeight := p.net[0][3].Sign() > 0
	for _, h := range p.net {
		if h[3].Cmp(p.net[0][3]) != 0 {
			constantWeight = false
			break
		}
	}
	if constantWeight {
		r.heightScale = new(big.Rat).Quo(scale, p.net[0][3])
		if e := b.check(r.heightScale); e != nil {
			return nil, e
		}
	}
	r.operations = b.operations
	return r, nil
}
